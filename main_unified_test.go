package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nwd-deakr/internal/adapters/benefitsindex"
	"nwd-deakr/internal/adapters/residentindex"
)

func TestUnifiedBothSourcesSucceed(t *testing.T) {
	residentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "R-10001",
			"first_name": "Ashley",
			"last_name": "Kessler"
		}`))
	}))
	defer residentServer.Close()

	benefitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`
<BenefitsRegister>
	<Record>
		<Ref>CA/2016/4001</Ref>
		<Name>KESSLER, Ashley</Name>
		<BenefitCode>HSP-A</BenefitCode>
	</Record>
</BenefitsRegister>
`))
	}))
	defer benefitServer.Close()

	residentClient := residentindex.NewClient(
		residentServer.URL,
		residentServer.Client(),
	)

	benefitsClient := benefitsindex.NewClient(
		benefitServer.URL,
		benefitServer.Client(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/unified?resident_id=R-10001&benefit_ref=CA/2016/4001",
		nil,
	)

	rec := httptest.NewRecorder()

	getUnified(residentClient, benefitsClient)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"id":"R-10001"`) {
		t.Fatalf("resident missing from response: %s", body)
	}

	if !strings.Contains(body, `"benefit_code":"HSP-A"`) {
		t.Fatalf("benefit missing from response: %s", body)
	}

	if !strings.Contains(body, `"partial":false`) {
		t.Fatalf("expected partial=false: %s", body)
	}
}

func TestUnifiedBenefitsUnavailable(t *testing.T) {
	residentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "R-10001",
			"first_name": "Ashley"
		}`))
	}))
	defer residentServer.Close()

	benefitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
	}))
	defer benefitServer.Close()

	residentClient := residentindex.NewClient(
		residentServer.URL,
		residentServer.Client(),
	)

	benefitsClient := benefitsindex.NewClient(
		benefitServer.URL,
		benefitServer.Client(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/unified?resident_id=R-10001&benefit_ref=CA/2016/4001",
		nil,
	)

	rec := httptest.NewRecorder()

	getUnified(residentClient, benefitsClient)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"id":"R-10001"`) {
		t.Fatalf("resident should still be returned: %s", body)
	}

	if !strings.Contains(body, `"benefits":null`) {
		t.Fatalf("benefits should be null: %s", body)
	}

	if !strings.Contains(body, `"partial":true`) {
		t.Fatalf("expected partial=true: %s", body)
	}

	if !strings.Contains(body, `"status":"unavailable"`) {
		t.Fatalf("expected unavailable status: %s", body)
	}
}

func TestUnifiedResidentUnavailable(t *testing.T) {
	residentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
	}))
	defer residentServer.Close()

	benefitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`
<BenefitsRegister>
	<Record>
		<Ref>CA/2016/4001</Ref>
		<Name>KESSLER, Ashley</Name>
		<BenefitCode>HSP-A</BenefitCode>
	</Record>
</BenefitsRegister>
`))
	}))
	defer benefitServer.Close()

	residentClient := residentindex.NewClient(
		residentServer.URL,
		residentServer.Client(),
	)

	benefitsClient := benefitsindex.NewClient(
		benefitServer.URL,
		benefitServer.Client(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/unified?resident_id=R-10001&benefit_ref=CA/2016/4001",
		nil,
	)

	rec := httptest.NewRecorder()

	getUnified(residentClient, benefitsClient)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"resident":null`) {
		t.Fatalf("resident should be null: %s", body)
	}

	if !strings.Contains(body, `"benefit_code":"HSP-A"`) {
		t.Fatalf("benefit should still be returned: %s", body)
	}

	if !strings.Contains(body, `"partial":true`) {
		t.Fatalf("expected partial=true: %s", body)
	}

	if !strings.Contains(body, `"status":"unavailable"`) {
		t.Fatalf("expected unavailable status: %s", body)
	}
}

func TestUnifiedBothSourcesUnavailable(t *testing.T) {
	residentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
	}))
	defer residentServer.Close()

	benefitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
	}))
	defer benefitServer.Close()

	residentClient := residentindex.NewClient(
		residentServer.URL,
		residentServer.Client(),
	)

	benefitsClient := benefitsindex.NewClient(
		benefitServer.URL,
		benefitServer.Client(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/unified?resident_id=R-10001&benefit_ref=CA/2016/4001",
		nil,
	)

	rec := httptest.NewRecorder()

	getUnified(residentClient, benefitsClient)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"resident":null`) {
		t.Fatalf("resident should be null: %s", body)
	}

	if !strings.Contains(body, `"benefits":null`) {
		t.Fatalf("benefits should be null: %s", body)
	}

	if !strings.Contains(body, `"partial":true`) {
		t.Fatalf("expected partial=true: %s", body)
	}

	if strings.Count(body, `"status":"unavailable"`) != 2 {
		t.Fatalf("expected both sources to be unavailable: %s", body)
	}
}
