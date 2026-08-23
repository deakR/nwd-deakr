package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nwd-deakr/internal/adapters/benefitsindex"
	"nwd-deakr/internal/adapters/residentindex"
	"nwd-deakr/internal/domain"
)

func TestHandlerGetResident(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain.Resident{ID: "R-10001", FirstName: "Ashley"})
	}))
	defer server.Close()

	client := residentindex.NewClient(server.URL, server.Client())
	req := httptest.NewRequest(http.MethodGet, "/residents/R-10001", nil)
	req.SetPathValue("id", "R-10001")
	rec := httptest.NewRecorder()

	getResident(client)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHandlerGetBenefit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<BenefitsRegister><Record><Ref>CA/2016/4001</Ref><BenefitCode>HSP-A</BenefitCode></Record></BenefitsRegister>`))
	}))
	defer server.Close()

	client := benefitsindex.NewClient(server.URL, server.Client())
	req := httptest.NewRequest(http.MethodGet, "/benefits/CA/2016/4001", nil)
	req.SetPathValue("ref", "CA/2016/4001")
	rec := httptest.NewRecorder()

	getBenefit(client)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"benefit_code":"HSP-A"`) {
		t.Fatalf("expected benefit code in response, got %s", rec.Body.String())
	}
}

func TestHandlerGetResidents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain.ResidentPage{
			Page: 1, PageSize: 25, Total: 1,
			Results: []domain.Resident{{ID: "R-10001", FirstName: "Ashley"}},
		})
	}))
	defer server.Close()

	client := residentindex.NewClient(server.URL, server.Client())
	req := httptest.NewRequest(http.MethodGet, "/residents", nil)
	rec := httptest.NewRecorder()

	getResidents(client)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"complete":true`) {
		t.Fatalf("expected complete catalogue in response, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"partial":true`) {
		t.Fatalf("did not expect partial flag on healthy source, got %s", rec.Body.String())
	}
}

func TestHandlerGetResidentsIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain.ResidentPage{
			Page: 1, PageSize: 25, Total: 2,
			Results: []domain.Resident{
				{ID: "R-10001", FirstName: "Ashley"},
				{ID: "R-10002", FirstName: "Maria"},
			},
		})
	}))
	defer server.Close()

	client := residentindex.NewClient(server.URL, server.Client())
	handler := getResidents(client)

	fetch := func() domain.ResidentListResponse {
		req := httptest.NewRequest(http.MethodGet, "/residents", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var response domain.ResidentListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return response
	}

	first := fetch()
	second := fetch()

	for _, response := range []domain.ResidentListResponse{first, second} {
		if len(response.Residents) != 2 {
			t.Fatalf("expected 2 residents, got %d", len(response.Residents))
		}
		status := response.Meta.Sources["resident_index"]
		status.LatencyMs = 0
		response.Meta.Sources["resident_index"] = status
	}

	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)

	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("same request must produce same result:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestHandlerGetResidentsDegradesWithoutErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := residentindex.NewClient(server.URL, server.Client())
	req := httptest.NewRequest(http.MethodGet, "/residents", nil)
	rec := httptest.NewRecorder()

	getResidents(client)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("aggregate endpoint must degrade with data + metadata, got status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"partial":true`) {
		t.Fatalf("expected partial flag when source fails, got %s", rec.Body.String())
	}
}
