package residentindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nwd-deakr/internal/domain"
)

func TestClientGetResidentSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/residents/R-10001" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain.Resident{
			ID:        "R-10001",
			FirstName: "Ashley",
			LastName:  "Kessler",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	resident, err := client.GetResident(context.Background(), "R-10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resident.ID != "R-10001" || resident.FirstName != "Ashley" {
		t.Fatalf("unexpected resident data: %+v", resident)
	}
}

func TestClientGetResidentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.GetResident(context.Background(), "R-999999")
	if err == nil || err.Error() != "resident not found" {
		t.Fatalf("expected 'resident not found', got %v", err)
	}
}

func TestClientGetResidentUpstream500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.GetResident(context.Background(), "R-10001")
	if err == nil {
		t.Fatalf("expected error on 500, got nil")
	}
}

func TestClientGetResidentInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.GetResident(context.Background(), "R-10001")
	if err == nil {
		t.Fatalf("expected error on invalid JSON, got nil")
	}
}
