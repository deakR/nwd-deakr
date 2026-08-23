package main

import (
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
