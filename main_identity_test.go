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

const catalogueXML = `<BenefitsRegister>
	<Record><Ref>NO/2019/4234</Ref><Name>DELGADO, Maria</Name><Born>1971-04-02</Born><Addr>118 Cedar Avenue</Addr><Town>Northgate</Town></Record>
</BenefitsRegister>`

func setupIdentityServers(t *testing.T, residentBody string, residentStatus int, benefitsHealthy bool) (*httptest.Server, *httptest.Server) {
	t.Helper()

	residentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(residentStatus)
		w.Write([]byte(residentBody))
	}))

	benefitsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !benefitsHealthy {
			http.Error(w, "SRV-500", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(catalogueXML))
	}))

	t.Cleanup(func() {
		residentServer.Close()
		benefitsServer.Close()
	})

	return residentServer, benefitsServer
}

func TestHandlerResidentUnifiedMatchesAcrossSources(t *testing.T) {
	residentServer, benefitsServer := setupIdentityServers(t,
		`{"id":"R-10234","first_name":"Maria","last_name":"Delgado","date_of_birth":"1971-04-02","address_line":"118 Cedar Ave","city":"Northgate"}`,
		http.StatusOK,
		true)

	residentClient := residentindex.NewClient(residentServer.URL, residentServer.Client())
	benefitsClient := benefitsindex.NewClient(benefitsServer.URL, benefitsServer.Client())
	catalogue := benefitsindex.NewCatalogue(benefitsClient, 0)

	req := httptest.NewRequest(http.MethodGet, "/residents/R-10234/unified", nil)
	req.SetPathValue("id", "R-10234")
	rec := httptest.NewRecorder()

	getResidentUnified(residentClient, catalogue)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response domain.AutoUnifiedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.IdentityMatch.Outcome != domain.IdentityMatched {
		t.Fatalf("expected matched, got %s (%+v)", response.IdentityMatch.Outcome, response.IdentityMatch)
	}
	if response.Benefits == nil || response.Benefits.Ref != "NO/2019/4234" {
		t.Fatalf("expected linked benefit record, got %+v", response.Benefits)
	}
	if response.Resident == nil || response.Resident.ID != "R-10234" {
		t.Fatalf("expected resident in response, got %+v", response.Resident)
	}
	if response.Meta.Partial {
		t.Fatalf("healthy sources must not be partial: %+v", response.Meta)
	}
	if len(response.IdentityMatch.Evidence) == 0 || response.IdentityMatch.Evidence[0].Rule != "exact_dob_and_name" {
		t.Fatalf("expected tier A evidence, got %+v", response.IdentityMatch.Evidence)
	}
}

func TestHandlerResidentUnifiedDegradesWhenCatalogueUnavailable(t *testing.T) {
	residentServer, benefitsServer := setupIdentityServers(t,
		`{"id":"R-10234","first_name":"Maria","last_name":"Delgado"}`,
		http.StatusOK,
		false)

	residentClient := residentindex.NewClient(residentServer.URL, residentServer.Client())
	benefitsClient := benefitsindex.NewClient(benefitsServer.URL, benefitsServer.Client())
	catalogue := benefitsindex.NewCatalogue(benefitsClient, 0)

	req := httptest.NewRequest(http.MethodGet, "/residents/R-10234/unified", nil)
	req.SetPathValue("id", "R-10234")
	rec := httptest.NewRecorder()

	getResidentUnified(residentClient, catalogue)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("identity endpoint must degrade with 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"outcome":"unavailable"`) {
		t.Fatalf("expected unavailable outcome, got %s", body)
	}
	if !strings.Contains(body, `"partial":true`) {
		t.Fatalf("expected partial=true, got %s", body)
	}
	if !strings.Contains(body, `"id":"R-10234"`) {
		t.Fatalf("resident data must still be returned, got %s", body)
	}
}

func TestHandlerResidentUnifiedNotFoundIs404(t *testing.T) {
	residentServer, benefitsServer := setupIdentityServers(t,
		`404 page not found`,
		http.StatusNotFound,
		true)

	residentClient := residentindex.NewClient(residentServer.URL, residentServer.Client())
	benefitsClient := benefitsindex.NewClient(benefitsServer.URL, benefitsServer.Client())
	catalogue := benefitsindex.NewCatalogue(benefitsClient, 0)

	req := httptest.NewRequest(http.MethodGet, "/residents/R-99999/unified", nil)
	req.SetPathValue("id", "R-99999")
	rec := httptest.NewRecorder()

	getResidentUnified(residentClient, catalogue)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown resident should 404 before matching, got %d", rec.Code)
	}
}
