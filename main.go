package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nwd-deakr/internal/adapters/benefitsindex"
	"nwd-deakr/internal/adapters/residentindex"
	"nwd-deakr/internal/domain"
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

func getResidents(client *residentindex.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		residents, pagination, status := client.GetResidents(r.Context())

		response := domain.ResidentListResponse{
			Residents:  residents,
			Pagination: pagination,
			Meta: domain.UnifiedMeta{
				Sources: map[string]domain.SourceStatus{
					"resident_index": status,
				},
				Partial: status.Status != "ok" || !pagination.Complete,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func getUnified(
	residentClient *residentindex.Client,
	benefitsClient *benefitsindex.Client,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		residentID := r.URL.Query().Get("resident_id")
		benefitRef := r.URL.Query().Get("benefit_ref")

		if residentID == "" && benefitRef == "" {
			http.Error(
				w,
				"resident_id or benefit_ref is required",
				http.StatusBadRequest,
			)
			return
		}

		type residentResult struct {
			data   *domain.Resident
			status domain.SourceStatus
		}

		type benefitResult struct {
			data   *domain.BenefitRecord
			status domain.SourceStatus
		}

		residentCh := make(chan residentResult, 1)
		benefitCh := make(chan benefitResult, 1)

		if residentID != "" {
			go func() {
				data, status := residentClient.GetResident(
					r.Context(),
					residentID,
				)

				residentCh <- residentResult{
					data:   data,
					status: status,
				}
			}()
		}

		if benefitRef != "" {
			go func() {
				data, status := benefitsClient.GetBenefit(
					r.Context(),
					benefitRef,
				)

				benefitCh <- benefitResult{
					data:   data,
					status: status,
				}
			}()
		}

		response := domain.UnifiedResponse{
			Meta: domain.UnifiedMeta{
				Sources: make(map[string]domain.SourceStatus),
			},
		}

		if residentID != "" {
			result := <-residentCh

			response.Resident = result.data
			response.Meta.Sources["resident_index"] = result.status

			if result.status.Status != "ok" {
				response.Meta.Partial = true
			}
		}

		if benefitRef != "" {
			result := <-benefitCh

			response.Benefits = result.data
			response.Meta.Sources["benefits_register"] = result.status

			if result.status.Status != "ok" {
				response.Meta.Partial = true
			}
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(response)
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
		"GET /residents",
		getResidents(residentClient),
	)

	mux.HandleFunc(
		"GET /benefits/{ref...}",
		getBenefit(benefitsClient),
	)

	mux.HandleFunc(
		"GET /unified",
		getUnified(residentClient, benefitsClient),
	)

	fmt.Println("Unified API running on http://127.0.0.1:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Server stopped:", err)
	}
}
