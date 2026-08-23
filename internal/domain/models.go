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
