package benefitsindex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientGetBenefitSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/records/CA/2016/4001" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`
<BenefitsRegister>
	<Record>
		<Ref>CA/2016/4001</Ref>
		<Name>KESSLER, Ashley</Name>
		<Born>1983-01-23</Born>
		<Addr>203 Hazel Street</Addr>
		<Town>Calder Central</Town>
		<BenefitCode>HSP-A</BenefitCode>
		<ReviewDue>2026-06-19</ReviewDue>
	</Record>
</BenefitsRegister>`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	record, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "ok" {
		t.Fatalf("unexpected status: %+v", status)
	}

	if record.Ref != "CA/2016/4001" || record.BenefitCode != "HSP-A" {
		t.Fatalf("unexpected benefit record: %+v", record)
	}
}

func TestClientGetBenefitRecoversOnRetryAfter500s(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			http.Error(w, "transient error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<BenefitsRegister><Record><Ref>CA/2016/4001</Ref><BenefitCode>HSP-A</BenefitCode></Record></BenefitsRegister>`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond

	record, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "ok" {
		t.Fatalf("expected ok after retry, got %+v", status)
	}
	if record == nil || record.Ref != "CA/2016/4001" {
		t.Fatalf("expected record, got nil")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts)
	}
}

func TestClientGetBenefitDoesNotRetry404(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, status := client.GetBenefit(context.Background(), "CA/2016/9999")
	if status.Status != "not_found" {
		t.Fatalf("expected not_found, got %s", status.Status)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Fatalf("404 must not be retried, attempts made: %d", attempts)
	}
}

func TestClientGetBenefitUpstream500ExhaustsRetries(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond

	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "unavailable" {
		t.Fatalf("expected unavailable, got %s", status.Status)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts before exhaustion, got %d", attempts)
	}
}

func TestGetAllBenefitsTripsSharedCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.MaxAttempts = 1
	client.RetryBackoff = 1 * time.Millisecond

	for i := range 3 {
		_, status := client.GetAllBenefits(context.Background())
		if status.Status != "unavailable" {
			t.Fatalf("call %d: expected unavailable, got %s", i+1, status.Status)
		}
	}

	_, status := client.GetAllBenefits(context.Background())
	if status.Status != "circuit_open" {
		t.Fatalf("expected circuit_open after 3 failures, got %s", status.Status)
	}
}
