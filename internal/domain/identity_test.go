package domain

import (
	"testing"
)

func residentForMatch() Resident {
	return Resident{
		ID:          "R-10234",
		FirstName:   "Maria",
		LastName:    "Delgado",
		DateOfBirth: "1971-04-02",
		AddressLine: "118 Cedar Ave",
		City:        "Northgate",
	}
}

func residentForMatchPtr() *Resident {
	r := residentForMatch()
	return &r
}

func benefitRecord(ref, name, born, addr, town string) BenefitRecord {
	return BenefitRecord{Ref: ref, Name: name, Born: born, Addr: addr, Town: town}
}

func TestMatchTierAExactDobAndName(t *testing.T) {
	catalogue := []BenefitRecord{
		benefitRecord("NO/2019/4234", "DELGADO, Maria", "1971-04-02", "9 Elsewhere", "Other Town"),
		benefitRecord("NO/2019/9999", "SMITH, John", "1980-01-01", "1 X St", "Y"),
	}

	record, meta := MatchResidentToCatalogue(residentForMatchPtr(), catalogue, 123)

	if meta.Outcome != IdentityMatched {
		t.Fatalf("expected matched, got %s (%+v)", meta.Outcome, meta)
	}
	if record == nil || record.Ref != "NO/2019/4234" {
		t.Fatalf("expected NO/2019/4234, got %+v", record)
	}
	if meta.MatchedRef != "NO/2019/4234" || len(meta.Evidence) != 1 || meta.Evidence[0].Rule != "exact_dob_and_name" {
		t.Fatalf("unexpected receipt: %+v", meta)
	}
}

func TestMatchTierBNameTownStreetWhenDobDiffers(t *testing.T) {
	resident := residentForMatch()
	catalogue := []BenefitRecord{
		benefitRecord("NO/2019/4234", "DELGADO, Maria", "1999-12-31", "118 Cedar Avenue", "Northgate"),
	}

	record, meta := MatchResidentToCatalogue(&resident, catalogue, 123)

	if meta.Outcome != IdentityMatched || record == nil || record.Ref != "NO/2019/4234" {
		t.Fatalf("expected tier B match via address normalization, got %s %+v", meta.Outcome, meta)
	}

	foundTierB := false
	for _, e := range meta.Evidence {
		if e.Rule == "name_town_street" {
			foundTierB = true
			break
		}
	}
	if !foundTierB {
		t.Fatalf("expected a name_town_street evidence entry, got %+v", meta.Evidence)
	}
	if len(meta.Evidence) < 2 {
		t.Fatalf("receipt should also record the failed tier A attempt, got %+v", meta.Evidence)
	}
}

func TestMatchAmbiguousTieDeclinesToMerge(t *testing.T) {
	resident := residentForMatch()
	catalogue := []BenefitRecord{
		benefitRecord("AS/2019/1107", "DELGADO, Maria", "1971-04-02", "1 A", "B"),
		benefitRecord("AS/2019/2245", "DELGADO, Maria", "1971-04-02", "2 C", "D"),
	}

	record, meta := MatchResidentToCatalogue(&resident, catalogue, 123)

	if meta.Outcome != IdentityAmbiguous {
		t.Fatalf("expected ambiguous on tie, got %s", meta.Outcome)
	}
	if record != nil {
		t.Fatalf("ambiguous must not merge, got %+v", record)
	}
	if len(meta.CandidateRefs) != 2 || meta.CandidateRefs[0] != "AS/2019/1107" || meta.CandidateRefs[1] != "AS/2019/2245" {
		t.Fatalf("tie must list every candidate: %+v", meta.CandidateRefs)
	}
}

func TestMatchNoMatchListsBothRulesAsTried(t *testing.T) {
	catalogue := []BenefitRecord{
		benefitRecord("NO/2019/0001", "PERSON, Other", "2000-01-01", "1 X St", "Y"),
	}

	_, meta := MatchResidentToCatalogue(residentForMatchPtr(), catalogue, 123)

	if meta.Outcome != IdentityNoMatch {
		t.Fatalf("expected no_match, got %s", meta.Outcome)
	}
	if len(meta.Evidence) != 2 ||
		meta.Evidence[0].Rule != "exact_dob_and_name" ||
		meta.Evidence[1].Rule != "name_town_street" {
		t.Fatalf("both tiers must be recorded as tried: %+v", meta.Evidence)
	}
}

func TestMatchUnavailableOnEmptyCatalogue(t *testing.T) {
	_, meta := MatchResidentToCatalogue(residentForMatchPtr(), nil, 0)

	if meta.Outcome != IdentityUnavailable {
		t.Fatalf("empty catalogue must be unavailable, not no_match; got %s", meta.Outcome)
	}
}

func TestNormalizationVariants(t *testing.T) {
	cases := []struct {
		a, b string
	}{
		{"118 Cedar Avenue", "118 Cedar Ave"},
		{"137 Poplar Road", "137 poplar rd"},
		{"12 Elm Street", "12 elm st"},
		{"8 Oak Drive", "8 oak dr"},
		{"9 Birch Lane", "9 birch ln"},
		{"O'Brien, Mary-Jane", "obrien maryjane"},
	}
	for _, c := range cases {
		if NormalizeStreet(c.a) != NormalizeStreet(c.b) && NormalizeName(c.a) != NormalizeName(c.b) {
			t.Fatalf("%q and %q should normalize equal (street=%q name=%q)", c.a, c.b, NormalizeStreet(c.a), NormalizeName(c.a))
		}
	}

	last, first := ParseLegacyName("DELGADO, Maria")
	if last != "delgado" || first != "maria" {
		t.Fatalf("legacy name parse wrong: %q / %q", last, first)
	}
}

func TestMatchGuardsAgainstEmptyIdentityFields(t *testing.T) {
	resident := Resident{ID: "R-1", FirstName: "", LastName: "", DateOfBirth: "", AddressLine: "", City: ""}
	catalogue := []BenefitRecord{
		benefitRecord("NO/2019/0001", ", ", "1971-04-02", "", ""),
	}

	_, meta := MatchResidentToCatalogue(&resident, catalogue, 123)

	if meta.Outcome == IdentityMatched {
		t.Fatal("empty resident identity fields must never match")
	}
}
