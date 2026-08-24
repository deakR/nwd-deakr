package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type groundTruthRestRow struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DateOfBirth string `json:"date_of_birth"`
	AddressLine string `json:"address_line"`
	City        string `json:"city"`
	PID         int    `json:"_pid"`
}

type groundTruthXmlRow struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
	Born string `json:"born"`
	Addr string `json:"addr"`
	Town string `json:"town"`
	PID  int    `json:"_pid"`
}

func loadGroundTruth(t *testing.T) ([]groundTruthRestRow, []BenefitRecord, map[string]int, int) {
	t.Helper()

	servicesDir := filepath.Join("..", "..", "references", "handbook, docs and data packs", "data pack and document", "data pack", "services")

	loadJSON := func(name string, target any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(servicesDir, name))
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
	}

	var restRows []groundTruthRestRow
	var xmlRows []groundTruthXmlRow
	loadJSON("_rest_data.json", &restRows)
	loadJSON("_xml_data.json", &xmlRows)

	if len(restRows) == 0 || len(xmlRows) == 0 {
		t.Fatalf("ground truth data unexpectedly empty: rest=%d xml=%d", len(restRows), len(xmlRows))
	}

	pidByRef := make(map[string]int, len(xmlRows))
	catalogue := make([]BenefitRecord, 0, len(xmlRows))
	for _, row := range xmlRows {
		pidByRef[row.Ref] = row.PID
		catalogue = append(catalogue, BenefitRecord{
			Ref:  row.Ref,
			Name: row.Name,
			Born: row.Born,
			Addr: row.Addr,
			Town: row.Town,
		})
	}

	xmlPids := make(map[int]bool, len(xmlRows))
	for _, pid := range pidByRef {
		xmlPids[pid] = true
	}
	overlap := 0
	for _, row := range restRows {
		if xmlPids[row.PID] {
			overlap++
		}
	}

	return restRows, catalogue, pidByRef, overlap
}

func TestIdentityMatchingGroundTruth(t *testing.T) {
	restRows, catalogue, pidByRef, overlap := loadGroundTruth(t)

	if overlap == 0 {
		t.Fatal("expected overlapping persons across sources in ground truth data")
	}

	matchedCorrect, matchedWrong, ambiguous, noMatch := 0, 0, 0, 0

	for i := range restRows {
		row := &restRows[i]
		resident := Resident{
			ID:          row.ID,
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			DateOfBirth: row.DateOfBirth,
			AddressLine: row.AddressLine,
			City:        row.City,
		}

		record, meta := MatchResidentToCatalogue(&resident, catalogue, 0)
		_ = record

		switch meta.Outcome {
		case IdentityMatched:
			truePID, ok := pidByRef[meta.MatchedRef]
			if !ok {
				matchedWrong++
				t.Errorf("resident %s matched unknown ref %s", resident.ID, meta.MatchedRef)
				continue
			}
			if truePID == row.PID {
				matchedCorrect++
			} else {
				matchedWrong++
				t.Errorf("WRONG MERGE resident %s (pid %d) -> %s (pid %d)", resident.ID, row.PID, meta.MatchedRef, truePID)
			}
		case IdentityAmbiguous:
			ambiguous++
		case IdentityNoMatch:
			noMatch++
		default:
			t.Fatalf("unexpected outcome %s for resident %s", meta.Outcome, resident.ID)
		}
	}

	recall := float64(matchedCorrect) / float64(overlap) * 100

	t.Logf("")
	t.Logf("=== Identity matching ground truth ===")
	t.Logf("catalogue size        : %d benefit records", len(catalogue))
	t.Logf("residents evaluated   : %d", len(restRows))
	t.Logf("overlap (truth pairs) : %d", overlap)
	t.Logf("matched correctly     : %d", matchedCorrect)
	t.Logf("matched WRONGLY       : %d", matchedWrong)
	t.Logf("ambiguous (declined)  : %d", ambiguous)
	t.Logf("no match              : %d", noMatch)
	t.Logf("PRECISION             : %.2f%%", float64(matchedCorrect)/float64(matchedCorrect+matchedWrong)*100)
	t.Logf("RECALL                : %.2f%% (%d of %d)", recall, matchedCorrect, overlap)

	if matchedWrong > 0 {
		t.Fatalf("precision violation: %d wrong merges — declining to merge must always beat wrong merges", matchedWrong)
	}
	if matchedCorrect == 0 {
		t.Fatal("matcher produced zero correct matches; tiers are broken")
	}
	fmt.Fprintln(os.Stderr, "")
}
