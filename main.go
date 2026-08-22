package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	// "strings"
	"time"
)

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

var client = &http.Client{
	Timeout: 5 * time.Second,
}

func getResident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		http.Error(w, "resident ID is required", http.StatusBadRequest)
		return
	}

	upstreamURL := "http://127.0.0.1:8081/residents/" + url.PathEscape(id)

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		upstreamURL,
		nil,
	)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "resident index unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "resident not found", http.StatusNotFound)
		return
	}

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("resident index returned HTTP %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	var resident Resident

	if err := json.NewDecoder(resp.Body).Decode(&resident); err != nil {
		http.Error(w, "invalid response from resident index", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resident)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /residents/{id}", getResident)

	fmt.Println("Unified API running on http://127.0.0.1:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Server stopped:", err)
	}
}
