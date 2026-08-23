package residentindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nwd-deakr/internal/domain"
)

func resident(id string) domain.Resident {
	return domain.Resident{
		ID:            id,
		FirstName:     "First-" + id,
		LastName:      "Last-" + id,
		ProgramStatus: "Active",
	}
}

func pageServer(t *testing.T, pages ...domain.ResidentPage) *httptest.Server {
	t.Helper()

	calls := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		n := calls
		calls++

		w.Header().Set("Content-Type", "application/json")

		if n >= len(pages) {
			json.NewEncoder(w).Encode(domain.ResidentPage{
				Page:     n + 1,
				PageSize: defaultPageSize,
			})
			return
		}

		json.NewEncoder(w).Encode(pages[n])
	}))
}

func TestGetResidentsDeduplicatesAcrossOverlappingPages(t *testing.T) {
	server := pageServer(t,
		domain.ResidentPage{Total: 7, HasMore: true, Results: []domain.Resident{
			resident("R-1"), resident("R-2"), resident("R-3"),
		}},
		domain.ResidentPage{Total: 7, HasMore: true, Results: []domain.Resident{
			resident("R-3"), resident("R-4"), resident("R-5"),
		}},
		domain.ResidentPage{Total: 7, HasMore: false, Results: []domain.Resident{
			resident("R-5"), resident("R-6"), resident("R-7"),
		}},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, status := client.GetResidents(context.Background())

	if status.Status != "ok" {
		t.Fatalf("expected ok, got %s (%s)", status.Status, status.ErrorMessage)
	}
	if !pagination.Complete || pagination.Reason != "" {
		t.Fatalf("expected complete catalogue, got %+v", pagination)
	}
	if pagination.Unique != 7 || pagination.RecordsSeen != 9 || pagination.Duplicates != 2 {
		t.Fatalf("unexpected receipt: %+v", pagination)
	}
	if pagination.PagesFetched != 3 || pagination.ReportedTotal != 7 {
		t.Fatalf("unexpected receipt: %+v", pagination)
	}

	wantOrder := []string{"R-1", "R-2", "R-3", "R-4", "R-5", "R-6", "R-7"}
	for i, want := range wantOrder {
		if residents[i].ID != want {
			t.Fatalf("expected first-seen order %v, got position %d = %s", wantOrder, i, residents[i].ID)
		}
	}
}

func TestGetResidentsCountsConflictsAndKeepsFirstSeen(t *testing.T) {
	changed := resident("R-1")
	changed.Phone = "555-0000"

	server := pageServer(t,
		domain.ResidentPage{Total: 2, HasMore: true, Results: []domain.Resident{
			resident("R-1"),
		}},
		domain.ResidentPage{Total: 2, HasMore: false, Results: []domain.Resident{
			changed, resident("R-2"),
		}},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, _ := client.GetResidents(context.Background())

	if pagination.Duplicates != 1 || pagination.Conflicts != 1 {
		t.Fatalf("expected 1 duplicate and 1 conflict, got %+v", pagination)
	}
	if len(residents) != 2 {
		t.Fatalf("expected 2 unique residents, got %d", len(residents))
	}
	if residents[0].Phone != "" {
		t.Fatalf("expected first-seen payload to win, got %+v", residents[0])
	}
}

func TestGetResidentsKeepsPartialDataWhenUpstreamFailsMidPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			json.NewEncoder(w).Encode(domain.ResidentPage{
				Total: 6, HasMore: true, Results: []domain.Resident{
					resident("R-1"), resident("R-2"),
				},
			})
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, status := client.GetResidents(context.Background())

	if status.Status != "unavailable" || status.HTTPCode != http.StatusInternalServerError {
		t.Fatalf("expected unavailable source, got %+v", status)
	}
	if pagination.Complete || pagination.Reason != "upstream_failure" {
		t.Fatalf("expected incomplete receipt, got %+v", pagination)
	}
	if pagination.Unique != 2 || pagination.PagesFetched != 1 {
		t.Fatalf("receipt must reflect collected data: %+v", pagination)
	}
	if len(residents) != 2 || residents[0].ID != "R-1" {
		t.Fatalf("partial data must still be returned, got %+v", residents)
	}
}

func TestGetResidentsStopsOnEmptyPageClaimingMore(t *testing.T) {
	server := pageServer(t,
		domain.ResidentPage{Total: 4, HasMore: true},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, status := client.GetResidents(context.Background())

	if pagination.Complete || pagination.Reason != "pagination_anomaly" {
		t.Fatalf("expected anomaly receipt, got %+v", pagination)
	}
	if status.Status != "unavailable" {
		t.Fatalf("expected unavailable source, got %s", status.Status)
	}
	if len(residents) != 0 || pagination.Unique != 0 {
		t.Fatalf("expected no residents, got %+v", pagination)
	}
}

func TestGetResidentsAbortsAtMaxPages(t *testing.T) {
	repeating := domain.ResidentPage{
		Total: 620, HasMore: true, Results: []domain.Resident{
			resident("R-1"), resident("R-2"),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repeating)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), MaxPages: 3}

	_, pagination, status := client.GetResidents(context.Background())

	if pagination.PagesFetched != 3 {
		t.Fatalf("expected 3 fetched pages, got %d", pagination.PagesFetched)
	}
	if pagination.Complete || pagination.Reason != "max_pages_reached" {
		t.Fatalf("expected max_pages_reached, got %+v", pagination)
	}
	if status.Status != "unavailable" {
		t.Fatalf("expected unavailable source, got %s", status.Status)
	}
}

func TestGetResidentsTreatsEmptyCatalogueAsComplete(t *testing.T) {
	server := pageServer(t,
		domain.ResidentPage{Total: 0, HasMore: false},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, status := client.GetResidents(context.Background())

	if status.Status != "ok" {
		t.Fatalf("expected ok, got %s", status.Status)
	}
	if !pagination.Complete || pagination.Reason != "" {
		t.Fatalf("empty catalogue must be complete, got %+v", pagination)
	}
	if pagination.Unique != 0 || pagination.ReportedTotal != 0 || len(residents) != 0 {
		t.Fatalf("unexpected receipt: %+v", pagination)
	}
}

func TestGetResidentsFlagsIncompleteCatalogueOnTotalMismatch(t *testing.T) {
	server := pageServer(t,
		domain.ResidentPage{Total: 5, HasMore: false, Results: []domain.Resident{
			resident("R-1"),
		}},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	residents, pagination, status := client.GetResidents(context.Background())

	if status.Status != "ok" {
		t.Fatalf("source itself was healthy, got %s", status.Status)
	}
	if pagination.Complete || pagination.Reason != "total_mismatch" {
		t.Fatalf("expected total_mismatch receipt, got %+v", pagination)
	}
	if len(residents) != 1 || pagination.Unique != 1 || pagination.ReportedTotal != 5 {
		t.Fatalf("unexpected receipt: %+v", pagination)
	}
}

func TestGetResidentsReportsContextCancellation(t *testing.T) {
	server := pageServer(t,
		domain.ResidentPage{Total: 25, HasMore: true, Results: []domain.Resident{
			resident("R-1"),
		}},
	)
	defer server.Close()

	client := NewClient(server.URL, server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, pagination, _ := client.GetResidents(ctx)

	if pagination.Complete || pagination.Reason != "context_canceled" {
		t.Fatalf("expected context_canceled receipt, got %+v", pagination)
	}
}
