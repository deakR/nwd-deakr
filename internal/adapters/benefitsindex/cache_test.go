package benefitsindex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testCacheRecordXML = `<BenefitsRegister><Record><Ref>CA/2016/4001</Ref><BenefitCode>HSP-A</BenefitCode></Record></BenefitsRegister>`

func newCacheTestClient(server *httptest.Server, ttl time.Duration) *Client {
	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	client.cache = NewCache(ttl)

	return client
}

func TestCacheServesRepeatReadsWithoutUpstream(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 5*time.Minute)

	for i := 0; i < 2; i++ {
		record, status := client.GetBenefit(context.Background(), "CA/2016/4001")
		if status.Status != "ok" {
			t.Fatalf("call %d: expected ok, got %s", i+1, status.Status)
		}
		if record == nil || record.Ref != "CA/2016/4001" {
			t.Fatalf("call %d: expected record, got %+v", i+1, record)
		}
	}

	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("second read must come from cache, upstream hits: %d", n)
	}
}

func TestCacheExpirySendsSecondRequestUpstream(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 30*time.Millisecond)

	client.GetBenefit(context.Background(), "CA/2016/4001")
	time.Sleep(60 * time.Millisecond)
	client.GetBenefit(context.Background(), "CA/2016/4001")

	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("expired entry must be refetched, upstream hits: %d", n)
	}
}

func TestStaleFallbackOnLiveFailureWithExpiredEntry(t *testing.T) {
	var healthy int32 = 1
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if atomic.LoadInt32(&healthy) == 0 {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 30*time.Millisecond)

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "ok" {
		t.Fatalf("warm-up: expected ok, got %s", status.Status)
	}

	atomic.StoreInt32(&healthy, 0)
	time.Sleep(60 * time.Millisecond)

	record, status := client.GetBenefit(context.Background(), "CA/2016/4001")

	if status.Status != "stale" {
		t.Fatalf("expected stale after live failure with expired cache entry, got %s", status.Status)
	}
	if record == nil || record.Ref != "CA/2016/4001" {
		t.Fatalf("stale response must carry the cached record, got %+v", record)
	}
}

func TestUnavailableWithoutCacheOnLiveFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newCacheTestClient(server, 5*time.Minute)

	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "unavailable" {
		t.Fatalf("expected unavailable without any cached entry, got %s", status.Status)
	}
}

func TestBreakerOpenServesStaleWhenCached(t *testing.T) {
	var healthy int32 = 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 0 {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 30*time.Millisecond)

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "ok" {
		t.Fatalf("warm-up: expected ok, got %s", status.Status)
	}

	atomic.StoreInt32(&healthy, 0)
	time.Sleep(60 * time.Millisecond)

	for i := 0; i < 3; i++ {
		client.breaker.RecordFailure()
	}

	if client.breaker.state != CircuitOpen {
		t.Fatalf("expected open circuit, got %d", client.breaker.state)
	}

	record, status := client.GetBenefit(context.Background(), "CA/2016/4001")

	if status.Status != "stale" {
		t.Fatalf("open circuit with cached entry should serve stale, got %s", status.Status)
	}
	if record == nil || record.Ref != "CA/2016/4001" {
		t.Fatalf("stale response must carry the cached record, got %+v", record)
	}
}

func TestBreakerOpenReturnsCircuitOpenWithoutCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newCacheTestClient(server, 5*time.Minute)

	for i := 0; i < 3; i++ {
		client.breaker.RecordFailure()
	}

	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "circuit_open" {
		t.Fatalf("open circuit without cache should return circuit_open, got %s", status.Status)
	}
}

func TestNotFoundDropsCachedEntryAndIsAuthoritative(t *testing.T) {
	var deleted int32

	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if atomic.LoadInt32(&deleted) == 1 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 30*time.Millisecond)

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "ok" {
		t.Fatalf("warm-up: expected ok, got %s", status.Status)
	}

	atomic.StoreInt32(&deleted, 1)
	time.Sleep(60 * time.Millisecond)

	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "not_found" {
		t.Fatalf("upstream 404 must win over cache, got %s", status.Status)
	}

	if _, found, _ := client.cache.get("CA/2016/4001"); found {
		t.Fatal("404 must drop the cached entry")
	}

	if _, status := client.GetBenefit(context.Background(), "CA/2016/4001"); status.Status != "not_found" {
		t.Fatalf("dropped entry must not resurrect as stale, got %s", status.Status)
	}
}

func TestConcurrentCacheAccessIsSafe(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCacheRecordXML))
	}))
	defer server.Close()

	client := newCacheTestClient(server, 5*time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ref := "CA/2016/4001"
			if n%4 == 0 {
				ref = "CA/2016/0000"
			}
			client.GetBenefit(context.Background(), ref)
		}(i)
	}
	wg.Wait()
}
