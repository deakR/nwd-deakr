package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nwd-deakr/internal/adapters/residentindex"
)

var residentIndexURL = "http://127.0.0.1:8081"

var client = &http.Client{
	Timeout: 5 * time.Second,
}

func getResident(client *residentindex.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if id == "" {
			http.Error(w, "resident ID is required", http.StatusBadRequest)
			return
		}

		resident, err := client.GetResident(r.Context(), id)
		if err != nil {
			if err.Error() == "resident not found" {
				http.Error(w, "resident not found", http.StatusNotFound)
				return
			}

			http.Error(w, "resident index unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resident)
	}
}

func main() {
	residentClient := residentindex.NewClient(
		residentIndexURL,
		client,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /residents/{id}",
		getResident(residentClient),
	)

	fmt.Println("Unified API running on http://127.0.0.1:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Server stopped:", err)
	}
}
