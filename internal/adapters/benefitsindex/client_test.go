package benefitsindex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestClientGetBenefitNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, status := client.GetBenefit(context.Background(), "CA/2016/9999")
	if status.Status != "not_found" {
		t.Fatalf("expected not_found, got %s", status.Status)
	}
}

func TestClientGetBenefitUpstream500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SRV-500", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, status := client.GetBenefit(context.Background(), "CA/2016/4001")
	if status.Status != "unavailable" {
		t.Fatalf("expected unavailable, got %s", status.Status)
	}
}
