package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nwd-deakr/internal/adapters/benefitsindex"
	"nwd-deakr/internal/adapters/residentindex"
)

var (
	residentIndexURL = "http://127.0.0.1:8081"
	benefitsURL      = "http://127.0.0.1:8082"
)

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

		resident, status := client.GetResident(r.Context(), id)

		if status.Status == "not_found" {
			http.Error(w, "resident not found", http.StatusNotFound)
			return
		}

		if status.Status != "ok" {
			http.Error(w, "resident index unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resident)
	}
}

func getBenefit(client *benefitsindex.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref := r.PathValue("ref")

		if ref == "" {
			http.Error(w, "benefit reference is required", http.StatusBadRequest)
			return
		}

		record, status := client.GetBenefit(r.Context(), ref)

		if status.Status == "not_found" {
			http.Error(w, "benefit record not found", http.StatusNotFound)
			return
		}

		if status.Status != "ok" {
			http.Error(w, "benefits register unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	}
}

func main() {
	residentClient := residentindex.NewClient(
		residentIndexURL,
		client,
	)

	benefitsClient := benefitsindex.NewClient(
		benefitsURL,
		client,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /residents/{id}",
		getResident(residentClient),
	)

	mux.HandleFunc(
		"GET /benefits/{ref...}",
		getBenefit(benefitsClient),
	)

	fmt.Println("Unified API running on http://127.0.0.1:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Server stopped:", err)
	}
}
