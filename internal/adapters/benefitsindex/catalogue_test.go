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

const testCatalogueXML = `<BenefitsRegister>
	<Record><Ref>NO/2019/4234</Ref><Name>DELGADO, Maria</Name><Born>1971-04-02</Born><Addr>118 Cedar Avenue</Addr><Town>Northgate</Town><BenefitCode>HSP-B</BenefitCode><ReviewDue>2026-05-14</ReviewDue></Record>
	<Record><Ref>AS/2024/4702</Ref><Name>EASTWOOD, Donna</Name><Born>1973-11-18</Born><Addr>137 Poplar Road</Addr><Town>Ash Hill</Town><BenefitCode>TRN-1</BenefitCode><ReviewDue>2026-06-25</ReviewDue></Record>
</BenefitsRegister>`

func TestCatalogueColdFetchAndFreshReuse(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCatalogueXML))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	catalogue := NewCatalogue(client, time.Hour)

	first, fresh, _ := catalogue.Get(context.Background())
	if !fresh || len(first) != 2 {
		t.Fatalf("cold fetch: fresh=%v len=%d", fresh, len(first))
	}

	second, fresh2, _ := catalogue.Get(context.Background())
	if !fresh2 || len(second) != 2 {
		t.Fatalf("warm read: fresh=%v len=%d", fresh2, len(second))
	}

	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("fresh reuse must not hit upstream, hits=%d", n)
	}
}

func TestCatalogueExpiryRefetches(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCatalogueXML))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	catalogue := NewCatalogue(client, 30*time.Millisecond)

	catalogue.Get(context.Background())
	time.Sleep(60 * time.Millisecond)
	_, fresh, _ := catalogue.Get(context.Background())

	if !fresh {
		t.Fatal("refetch after expiry should be fresh")
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("expected 2 upstream fetches, got %d", n)
	}
}

func TestCatalogueServesStaleSnapshotOnFailure(t *testing.T) {
	var healthy int32 = 1
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if atomic.LoadInt32(&healthy) == 0 {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCatalogueXML))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	catalogue := NewCatalogue(client, 30*time.Millisecond)

	catalogue.Get(context.Background())

	atomic.StoreInt32(&healthy, 0)
	time.Sleep(60 * time.Millisecond)

	records, fresh, fetchedAtMs := catalogue.Get(context.Background())

	if records == nil || len(records) != 2 {
		t.Fatalf("stale snapshot must still serve data, got %d records", len(records))
	}
	if fresh {
		t.Fatal("served-after-failure snapshot must be flagged stale")
	}
	if fetchedAtMs == 0 {
		t.Fatal("stale snapshot must retain its original fetch timestamp")
	}
}

func TestCatalogueUnavailableWithoutAnySnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	catalogue := NewCatalogue(client, time.Hour)

	records, fresh, _ := catalogue.Get(context.Background())

	if records != nil || fresh {
		t.Fatalf("no snapshot and failed fetch must return nil, got %v fresh=%v", records, fresh)
	}
}

func TestCatalogueSingleFlightUnderConcurrency(t *testing.T) {
	var hits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testCatalogueXML))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.RetryBackoff = 1 * time.Millisecond
	catalogue := NewCatalogue(client, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			catalogue.Get(context.Background())
		}()
	}
	wg.Wait()

	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("concurrent cold reads must single-flight into one fetch, hits=%d", n)
	}
}
