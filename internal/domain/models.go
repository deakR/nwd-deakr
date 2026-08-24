package domain

type Resident struct {
	ID            string `json:"id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	DateOfBirth   string `json:"date_of_birth"`
	AddressLine   string `json:"address_line"`
	City          string `json:"city"`
	Phone         string `json:"phone"`
	ProgramStatus string `json:"program_status"`
	LastContact   string `json:"last_contact"`
}

type BenefitRecord struct {
	Ref         string `json:"ref"`
	Name        string `json:"name"`
	Born        string `json:"born"`
	Addr        string `json:"addr"`
	Town        string `json:"town"`
	BenefitCode string `json:"benefit_code"`
	ReviewDue   string `json:"review_due"`
}

type SourceStatus struct {
	Status       string `json:"status"`
	HTTPCode     int    `json:"http_code,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type UnifiedResponse struct {
	Resident *Resident      `json:"resident"`
	Benefits *BenefitRecord `json:"benefits"`
	Meta     UnifiedMeta    `json:"_meta"`
}

type UnifiedMeta struct {
	Sources map[string]SourceStatus `json:"sources"`
	Partial bool                    `json:"partial"`
}

type ResidentPage struct {
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
	HasMore  bool       `json:"has_more"`
	Results  []Resident `json:"results"`
}

type PaginationStatus struct {
	PagesFetched  int    `json:"pages_fetched"`
	RecordsSeen   int    `json:"records_seen"`
	Duplicates    int    `json:"duplicates"`
	Conflicts     int    `json:"conflicts"`
	Unique        int    `json:"unique"`
	ReportedTotal int    `json:"reported_total"`
	Complete      bool   `json:"complete"`
	Reason        string `json:"reason,omitempty"`
}

type ResidentListResponse struct {
	Residents  []Resident       `json:"residents"`
	Pagination PaginationStatus `json:"pagination"`
	Meta       UnifiedMeta      `json:"_meta"`
}

const (
	IdentityMatched     = "matched"
	IdentityAmbiguous   = "ambiguous"
	IdentityNoMatch     = "no_match"
	IdentityUnavailable = "unavailable"
)

type IdentityEvidence struct {
	Rule          string `json:"rule"`
	ResidentValue string `json:"resident_value,omitempty"`
	BenefitValue  string `json:"benefit_value,omitempty"`
	Result        string `json:"result,omitempty"`
}

type IdentityMatchMeta struct {
	Outcome              string             `json:"outcome"`
	MatchedRef           string             `json:"matched_ref,omitempty"`
	CandidateRefs        []string           `json:"candidate_refs"`
	Evidence             []IdentityEvidence `json:"evidence"`
	CatalogueFetchedAtMs int64              `json:"catalogue_fetched_at_ms"`
}

type AutoUnifiedResponse struct {
	Resident      *Resident         `json:"resident"`
	Benefits      *BenefitRecord    `json:"benefits"`
	IdentityMatch IdentityMatchMeta `json:"identity_match"`
	Meta          UnifiedMeta       `json:"_meta"`
}
