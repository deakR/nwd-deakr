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
