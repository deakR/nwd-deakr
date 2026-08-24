package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func NormalizeName(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), ""), "")
}

func NormalizeTown(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), ""), "")
}

var streetSuffixes = []struct{ long, short string }{
	{"avenue", "ave"},
	{"street", "st"},
	{"road", "rd"},
	{"drive", "dr"},
	{"lane", "ln"},
}

func NormalizeStreet(s string) string {
	s = strings.ToLower(s)
	for _, suffix := range streetSuffixes {
		s = strings.ReplaceAll(s, suffix.long, suffix.short)
	}
	return strings.Trim(nonAlnum.ReplaceAllString(s, ""), "")
}

func ParseLegacyName(name string) (last, first string) {
	if i := strings.Index(name, ","); i >= 0 {
		return NormalizeName(name[:i]), NormalizeName(name[i+1:])
	}
	return "", NormalizeName(name)
}

type matchCandidate struct {
	record *BenefitRecord
	dob    string
	last   string
	first  string
	town   string
	street string
}

func MatchResidentToCatalogue(resident *Resident, catalogue []BenefitRecord, catalogueFetchedAtMs int64) (*BenefitRecord, IdentityMatchMeta) {
	meta := IdentityMatchMeta{
		Outcome:              IdentityUnavailable,
		CandidateRefs:        []string{},
		Evidence:             []IdentityEvidence{},
		CatalogueFetchedAtMs: catalogueFetchedAtMs,
	}

	if len(catalogue) == 0 {
		return nil, meta
	}

	meta.Outcome = IdentityNoMatch

	if resident == nil {
		return nil, meta
	}

	rLast := NormalizeName(resident.LastName)
	rFirst := NormalizeName(resident.FirstName)
	rDOB := strings.TrimSpace(resident.DateOfBirth)
	rTown := NormalizeTown(resident.City)
	rStreet := NormalizeStreet(resident.AddressLine)

	nameUsable := rFirst != "" || rLast != ""

	candidates := make([]matchCandidate, 0, len(catalogue))
	for i := range catalogue {
		b := &catalogue[i]
		bLast, bFirst := ParseLegacyName(b.Name)

		candidates = append(candidates, matchCandidate{
			record: b,
			dob:    strings.TrimSpace(b.Born),
			last:   bLast,
			first:  bFirst,
			town:   NormalizeTown(b.Town),
			street: NormalizeStreet(b.Addr),
		})
	}

	if nameUsable && rDOB != "" {
		var tierA []*matchCandidate

		for i := range candidates {
			c := &candidates[i]
			if c.dob == rDOB && c.first == rFirst && c.last == rLast {
				tierA = append(tierA, c)
			}
		}

		outcome, matched, evidence := resolveTier("exact_dob_and_name", tierA,
			fmt.Sprintf("%s | %s %s", rDOB, rFirst, rLast),
			func(c *matchCandidate) string {
				return fmt.Sprintf("%s | %s %s", c.dob, c.first, c.last)
			},
		)

		meta.Evidence = append(meta.Evidence, evidence...)

		if outcome == IdentityMatched {
			meta.Outcome = IdentityMatched
			meta.MatchedRef = matched.record.Ref
			meta.CandidateRefs = []string{matched.record.Ref}
			return matched.record, meta
		}

		if outcome == IdentityAmbiguous {
			meta.Outcome = IdentityAmbiguous
			for _, c := range tierA {
				meta.CandidateRefs = append(meta.CandidateRefs, c.record.Ref)
			}
			return nil, meta
		}
	}

	if nameUsable && rFirst != "" && rLast != "" && rTown != "" && rStreet != "" {
		var tierB []*matchCandidate

		for i := range candidates {
			c := &candidates[i]
			if c.first == rFirst && c.last == rLast && c.town == rTown && c.street == rStreet {
				tierB = append(tierB, c)
			}
		}

		outcome, matched, evidence := resolveTier("name_town_street", tierB,
			fmt.Sprintf("%s %s | %s | %s", rFirst, rLast, rTown, rStreet),
			func(c *matchCandidate) string {
				return fmt.Sprintf("%s %s | %s | %s", c.first, c.last, c.town, c.street)
			},
		)

		meta.Evidence = append(meta.Evidence, evidence...)

		if outcome == IdentityMatched {
			meta.Outcome = IdentityMatched
			meta.MatchedRef = matched.record.Ref
			meta.CandidateRefs = []string{matched.record.Ref}
			return matched.record, meta
		}

		if outcome == IdentityAmbiguous {
			meta.Outcome = IdentityAmbiguous
			for _, c := range tierB {
				meta.CandidateRefs = append(meta.CandidateRefs, c.record.Ref)
			}
			return nil, meta
		}
	}

	return nil, meta
}

func resolveTier(rule string, tier []*matchCandidate, residentValue string, benefitValue func(*matchCandidate) string) (string, *matchCandidate, []IdentityEvidence) {
	switch {
	case len(tier) == 1:
		return IdentityMatched, tier[0], []IdentityEvidence{{
			Rule:          rule,
			ResidentValue: residentValue,
			BenefitValue:  benefitValue(tier[0]),
		}}

	case len(tier) > 1:
		return IdentityAmbiguous, nil, []IdentityEvidence{{
			Rule:          rule,
			ResidentValue: residentValue,
			Result:        fmt.Sprintf("%d candidates matched identically; declining to merge", len(tier)),
		}}

	default:
		return IdentityNoMatch, nil, []IdentityEvidence{{
			Rule:          rule,
			ResidentValue: residentValue,
			Result:        "0 candidates",
		}}
	}
}
