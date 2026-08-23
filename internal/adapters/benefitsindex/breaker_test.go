package benefitsindex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testRecordXML = `<BenefitsRegister><Record><Ref>CA/2016/4001</Ref><BenefitCode>HSP-A</BenefitCode></Record></BenefitsRegister>`

func newBreakerTestClient(server *httptest.Server, failureLimit int, cooldown time.Duration) *Client {
	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	client.MaxAttempts = 1
	client.breaker = NewCircuitBreaker(failureLimit, cooldown)

	return client
}

func TestCircuitBreakerStaysClosedOnSuccessfulOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRecordXML))
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 3, 5*time.Second)

	for i := 0; i < 5; i++ {
		ref := fmt.Sprintf("CA/2016/%04d", i)
		if _, status := client.GetBenefit(context.Background(), ref); status.Status != "ok" {
			t.Fatalf("expected ok on operation %d, got %s", i+1, status.Status)
		}
	}

	if client.breaker.state != CircuitClosed {
		t.Fatalf("expected circuit to remain closed, got %d", client.breaker.state)
	}
	if client.breaker.failures != 0 {
		t.Fatalf("expected zero recorded failures, got %d", client.breaker.failures)
	}
}

func TestCircuitOpensAfterThreeConsecutiveFailedOperations(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 3, 5*time.Second)

	for i := 0; i < 3; i++ {
		_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
		if status.Status != "unavailable" {
			t.Fatalf("operation %d: expected unavailable before the breaker opens, got %s", i+1, status.Status)
		}
	}

	if client.breaker.state != CircuitOpen {
		t.Fatalf("expected circuit to open after 3 failed operations, got state %d", client.breaker.state)
	}

	before := atomic.LoadInt32(&attempts)
	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")

	if status.Status != "circuit_open" {
		t.Fatalf("expected circuit_open once tripped, got %s", status.Status)
	}
	if atomic.LoadInt32(&attempts) != before {
		t.Fatalf("open circuit must not call upstream: attempts went %d -> %d", before, attempts)
	}
}

func TestOpenCircuitRejectsWithoutCallingUpstream(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 2, 5*time.Second)

	for i := 0; i < 2; i++ {
		client.GetBenefit(context.Background(), "CA/2016/4001")
	}

	if client.breaker.state != CircuitOpen {
		t.Fatalf("expected open circuit, got %d", client.breaker.state)
	}

	before := atomic.LoadInt32(&hits)
	for i := 0; i < 4; i++ {
		_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
		if status.Status != "circuit_open" {
			t.Fatalf("rejection %d: expected circuit_open, got %s", i+1, status.Status)
		}
	}

	if after := atomic.LoadInt32(&hits); after != before {
		t.Fatalf("upstream called while circuit open: %d -> %d", before, after)
	}
}

func TestCircuitRemainsOpenBeforeCooldownElapses(t *testing.T) {
	breaker := NewCircuitBreaker(2, 150*time.Millisecond)

	breaker.RecordFailure()
	breaker.RecordFailure()

	if allowed, isTrial := breaker.Allow(); allowed {
		t.Fatalf("circuit must reject immediately after opening (allowed=%v, isTrial=%v)", allowed, isTrial)
	}

	time.Sleep(60 * time.Millisecond)

	if allowed, _ := breaker.Allow(); allowed {
		t.Fatal("circuit must stay open until the full cooldown elapses")
	}
}

func TestHalfOpenAllowsSingleTrialAfterCooldown(t *testing.T) {
	breaker := NewCircuitBreaker(1, 50*time.Millisecond)

	breaker.RecordFailure()

	time.Sleep(70 * time.Millisecond)

	allowed, isTrial := breaker.Allow()
	if !allowed || !isTrial {
		t.Fatalf("after cooldown exactly one trial should be allowed, got allowed=%v isTrial=%v", allowed, isTrial)
	}

	if allowed, _ := breaker.Allow(); allowed {
		t.Fatal("second concurrent candidate must be rejected while the trial is in flight")
	}
}

func TestHalfOpenTrialSuccessClosesCircuit(t *testing.T) {
	breaker := NewCircuitBreaker(1, 40*time.Millisecond)

	breaker.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	if allowed, isTrial := breaker.Allow(); !allowed || !isTrial {
		t.Fatalf("expected a trial slot, got allowed=%v isTrial=%v", allowed, isTrial)
	}

	breaker.RecordSuccess()

	if breaker.state != CircuitClosed {
		t.Fatalf("trial success must close the circuit, got %d", breaker.state)
	}
	if breaker.failures != 0 {
		t.Fatalf("trial success must reset the failure counter, got %d", breaker.failures)
	}
	if allowed, isTrial := breaker.Allow(); !allowed || isTrial {
		t.Fatalf("closed circuit must allow normal requests, got allowed=%v isTrial=%v", allowed, isTrial)
	}
}

func TestHalfOpenTrialFailureReopensCircuit(t *testing.T) {
	breaker := NewCircuitBreaker(1, 40*time.Millisecond)

	breaker.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	if allowed, isTrial := breaker.Allow(); !allowed || !isTrial {
		t.Fatalf("expected a trial slot, got allowed=%v isTrial=%v", allowed, isTrial)
	}

	breaker.RecordFailure()

	if breaker.state != CircuitOpen {
		t.Fatalf("failed trial must reopen the circuit, got %d", breaker.state)
	}

	if allowed, _ := breaker.Allow(); allowed {
		t.Fatal("a fresh cooldown must start after a failed trial")
	}

	time.Sleep(60 * time.Millisecond)

	if allowed, isTrial := breaker.Allow(); !allowed || !isTrial {
		t.Fatalf("after the fresh cooldown another trial must be possible, got allowed=%v isTrial=%v", allowed, isTrial)
	}
}

func TestNotFoundDoesNotCountAsCircuitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 2, 5*time.Second)

	for i := 0; i < 4; i++ {
		if _, status := client.GetBenefit(context.Background(), "CA/2016/9999"); status.Status != "not_found" {
			t.Fatalf("operation %d: expected not_found, got %s", i+1, status.Status)
		}
	}

	if client.breaker.state != CircuitClosed {
		t.Fatalf("404 responses must never trip the circuit, got state %d", client.breaker.state)
	}
	if client.breaker.failures != 0 {
		t.Fatalf("404 responses must not increment the failure counter, got %d", client.breaker.failures)
	}
}

func TestOperationLevelCountingRetriedRecoveryIsNotAFailure(t *testing.T) {
	var requests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := atomic.AddInt32(&requests, 1); n%2 == 1 {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRecordXML))
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 2, 5*time.Second)
	client.MaxAttempts = 2

	for i := 0; i < 5; i++ {
		ref := fmt.Sprintf("CA/2016/%04d", i)
		if _, status := client.GetBenefit(context.Background(), ref); status.Status != "ok" {
			t.Fatalf("operation %d: expected ok after retry, got %s", i+1, status.Status)
		}
	}

	if client.breaker.state != CircuitClosed {
		t.Fatalf("operations that recover on retry must not trip the circuit, got state %d", client.breaker.state)
	}
	if client.breaker.failures != 0 {
		t.Fatalf("attempt-level 500s must not be counted as circuit failures, got %d", client.breaker.failures)
	}
}

func TestConcurrentSingleFlightInHalfOpen(t *testing.T) {
	breaker := NewCircuitBreaker(1, 60*time.Millisecond)

	breaker.RecordFailure()

	time.Sleep(90 * time.Millisecond)

	const candidates = 25

	type verdict struct {
		allowed bool
		isTrial bool
	}

	results := make(chan verdict, candidates)

	var wg sync.WaitGroup
	for i := 0; i < candidates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, isTrial := breaker.Allow()
			results <- verdict{allowed: allowed, isTrial: isTrial}
		}()
	}
	wg.Wait()
	close(results)

	trials, passes := 0, 0
	for v := range results {
		if v.allowed {
			passes++
		}
		if v.allowed && v.isTrial {
			trials++
		}
	}

	if passes != 1 || trials != 1 {
		t.Fatalf("exactly one goroutine may pass through HALF-OPEN, got passes=%d trials=%d", passes, trials)
	}
}

func TestClientRecoversThroughHalfOpenTrial(t *testing.T) {
	var healthy int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 0 {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRecordXML))
	}))
	defer server.Close()

	client := newBreakerTestClient(server, 3, 60*time.Millisecond)

	for i := 0; i < 3; i++ {
		if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "unavailable" {
			t.Fatalf("warming failures: expected unavailable, got %s", status.Status)
		}
	}

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "circuit_open" {
		t.Fatalf("expected circuit_open while open, got %s", status.Status)
	}

	atomic.StoreInt32(&healthy, 1)
	time.Sleep(90 * time.Millisecond)

	record, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "ok" || record == nil {
		t.Fatalf("half-open trial should succeed against a recovered upstream, got %s", status.Status)
	}

	if client.breaker.state != CircuitClosed {
		t.Fatalf("successful trial must close the circuit, got %d", client.breaker.state)
	}

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "ok" {
		t.Fatalf("closed circuit should serve normally again, got %s", status.Status)
	}
}
