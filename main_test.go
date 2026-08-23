package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nwd-deakr/internal/adapters/residentindex"
	"nwd-deakr/internal/domain"
	"testing"
)

func TestGetResidentSuccess(t *testing.T) {
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

	testClient := residentindex.NewClient(
		server.URL,
		&http.Client{},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/residents/R-10001",
		nil,
	)

	req.SetPathValue("id", "R-10001")

	rec := httptest.NewRecorder()

	getResident(testClient)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resident domain.Resident

	if err := json.NewDecoder(rec.Body).Decode(&resident); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resident.ID != "R-10001" {
		t.Fatalf("expected resident R-10001, got %s", resident.ID)
	}
}

func TestGetResidentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	testClient := residentindex.NewClient(
		server.URL,
		&http.Client{},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/residents/R-999999",
		nil,
	)

	req.SetPathValue("id", "R-999999")

	rec := httptest.NewRecorder()

	getResident(testClient)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetResidentUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()

	testClient := residentindex.NewClient(
		server.URL,
		&http.Client{},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/residents/R-10001",
		nil,
	)

	req.SetPathValue("id", "R-10001")

	rec := httptest.NewRecorder()

	getResident(testClient)(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestGetResidentInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`this is not valid JSON`))
	}))
	defer server.Close()

	testClient := residentindex.NewClient(
		server.URL,
		&http.Client{},
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/residents/R-10001",
		nil,
	)

	req.SetPathValue("id", "R-10001")

	rec := httptest.NewRecorder()

	getResident(testClient)(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}
