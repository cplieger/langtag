package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag"
)

// TestIntelligibleFloorAdmitsNoSharedLiteracy is the test that makes the
// package's central promise checkable rather than merely stated.
//
// A caller at the intelligible floor is told it will accept languages that are
// close to each other, and nothing that rests on a population happening to be
// literate in a second language. The first version of this library shipped a
// Catalan-to-Spanish entry on that tier, which is a shared-literacy claim
// wearing an interchangeability label, and two independent reviews caught it
// only by reading the reasons. This test catches it from the types.
func TestIntelligibleFloorAdmitsNoSharedLiteracy(t *testing.T) {
	t.Parallel()
	for _, f := range langtag.Fallbacks() {
		if f.Kind != langtag.SharedLiteracy {
			continue
		}
		want, have := langtag.MustParse(f.Want), langtag.MustParse(f.Have)
		if langtag.Match(want, have, langtag.TierIntelligible) {
			t.Errorf("Match(%q, %q, intelligible) = true, want false; %q is a shared-literacy entry and must need its own opt-in",
				f.Want, f.Have, f.Want+"->"+f.Have)
		}
		if !langtag.Match(want, have, langtag.TierSharedLiteracy) {
			t.Errorf("Match(%q, %q, shared-literacy) = false, want true", f.Want, f.Have)
		}
	}
}

// TestFallbackKindMatchesDirection pins the invariant that gives the two tiers
// their meaning. Interchangeability is symmetric, because that is what the claim
// says. Shared literacy is not: a minority-language population reads the
// majority language and the majority population does not reciprocate. An entry
// violating this is making the weaker claim under the stronger name.
func TestFallbackKindMatchesDirection(t *testing.T) {
	t.Parallel()
	for _, f := range langtag.Fallbacks() {
		switch f.Kind {
		case langtag.Intelligible:
			if !f.Both {
				t.Errorf("entry %s->%s is Intelligible but Both is false; interchangeability runs both ways, so this is a shared-literacy claim mislabelled",
					f.Want, f.Have)
			}
		case langtag.SharedLiteracy:
			if f.Both {
				t.Errorf("entry %s->%s is SharedLiteracy but Both is true; a majority-language population does not reciprocate",
					f.Want, f.Have)
			}
		default:
			t.Errorf("entry %s->%s has unknown Kind %d", f.Want, f.Have, f.Kind)
		}
	}
}

// TestSharedLiteracyIsOneWay covers a well-formed one-way entry: it applies in
// the stated direction and not in reverse.
func TestSharedLiteracyIsOneWay(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "ca", Have: "es", Kind: langtag.SharedLiteracy, Reason: "test"},
	})
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	if got := c.Compare(ca, es); got != langtag.TierSharedLiteracy {
		t.Errorf("Compare(ca, es) = %v, want %v", got, langtag.TierSharedLiteracy)
	}
	if got := c.Compare(es, ca); got != langtag.TierNone {
		t.Errorf("Compare(es, ca) = %v, want %v (a shared-literacy claim is not reciprocated)",
			got, langtag.TierNone)
	}
}

// TestMalformedFallbacksAreDropped is the fix for the fail-open defect two
// reviewers found independently.
//
// Kind decides the tier, and its zero value used to be Intelligible, the
// stronger of the two claims. So a caller-supplied entry that simply forgot to
// set Kind landed a one-way relationship on the tier reserved for symmetric
// ones, which is the exact error the Kind field exists to prevent. A malformed
// entry now removes a substitution rather than licensing an unintended one.
func TestMalformedFallbacksAreDropped(t *testing.T) {
	t.Parallel()
	cases := map[string]langtag.Fallback{
		"kind unset":            {Want: "sv", Have: "no", Reason: "r", Both: false},
		"kind unset but both":   {Want: "sv", Have: "no", Reason: "r", Both: true},
		"unknown kind":          {Want: "sv", Have: "no", Reason: "r", Kind: langtag.Kind(9), Both: true},
		"intelligible one way":  {Want: "sv", Have: "no", Reason: "r", Kind: langtag.Intelligible},
		"shared literacy both":  {Want: "sv", Have: "no", Reason: "r", Kind: langtag.SharedLiteracy, Both: true},
		"blank want":            {Want: "", Have: "no", Reason: "r", Kind: langtag.Intelligible, Both: true},
		"blank have":            {Want: "sv", Have: "", Reason: "r", Kind: langtag.Intelligible, Both: true},
		"same language on both": {Want: "sv", Have: "sv", Reason: "r", Kind: langtag.Intelligible, Both: true},
	}
	sv, no := langtag.MustParse("sv"), langtag.MustParse("no")
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := langtag.WithFallbacks([]langtag.Fallback{f})
			if got := c.Compare(sv, no); got != langtag.TierNone {
				t.Errorf("Compare(sv, no) with a malformed entry = %v, want %v", got, langtag.TierNone)
			}
			if errs := langtag.ValidateFallbacks([]langtag.Fallback{f}); len(errs) != 1 {
				t.Errorf("ValidateFallbacks returned %d errors, want 1: %v", len(errs), errs)
			}
		})
	}
}

func TestValidateFallbacksAcceptsTheBuiltInTable(t *testing.T) {
	t.Parallel()
	if errs := langtag.ValidateFallbacks(langtag.Fallbacks()); errs != nil {
		t.Errorf("ValidateFallbacks(Fallbacks()) = %v, want nil; the shipped table must satisfy its own contract", errs)
	}
}

// TestUnusableFloorSelectsNothing is the second half of the fail-open fix.
//
// TierNone means "no relationship", so treating it as a floor accepted every
// language as a substitute for every other. It is also what ParseTier returns
// for input it does not recognise, so a mistyped configuration value was the
// most permissive setting available rather than the least.
func TestUnusableFloorSelectsNothing(t *testing.T) {
	t.Parallel()
	ta, en := langtag.MustParse("ta"), langtag.MustParse("en")
	var zero langtag.Tag

	for _, floor := range []langtag.Tier{langtag.TierNone, langtag.Tier(99)} {
		if langtag.Match(ta, en, floor) {
			t.Errorf("Match(ta, en, %v) = true, want false", floor)
		}
		if langtag.Match(zero, zero, floor) {
			t.Errorf("Match(zero, zero, %v) = true, want false", floor)
		}
		got, tier, ok := langtag.Best(langtag.MustParse("ca"),
			[]langtag.Tag{langtag.MustParse("es")},
			func(t langtag.Tag) langtag.Tag { return t }, floor)
		if ok {
			t.Errorf("Best(ca, [es], %v) = (%v, %v, true), want ok=false; an unusable floor must not widen to the most permissive tier",
				floor, got, tier)
		}
	}
}

// TestMistypedTierFailsClosed joins the two halves: the value ParseTier hands
// back on failure must be the least permissive floor, not the most.
func TestMistypedTierFailsClosed(t *testing.T) {
	t.Parallel()
	floor, ok := langtag.ParseTier("looose")
	if ok {
		t.Fatalf("ParseTier(%q) ok = true, want false", "looose")
	}
	if langtag.Match(langtag.MustParse("ta"), langtag.MustParse("en"), floor) {
		t.Error("a caller that ignored ParseTier's ok value got a floor that matches everything, want one that matches nothing")
	}
}

// TestFallbackEntriesCarryProvenance keeps the table auditable. The first
// version shipped a provenance claim ("CLDR carries no relation between the
// two") that was false against upstream, which is why provenance is now a field
// a reviewer can check rather than prose buried in a reason.
func TestFallbackEntriesCarryProvenance(t *testing.T) {
	t.Parallel()
	for _, f := range langtag.Fallbacks() {
		if f.Provenance == "" {
			t.Errorf("entry %s->%s has an empty Provenance; a claim nobody can re-check is a claim nobody can maintain",
				f.Want, f.Have)
		}
		if f.Reason == "" {
			t.Errorf("entry %s->%s has an empty Reason", f.Want, f.Have)
		}
	}
}

func TestKindTier(t *testing.T) {
	t.Parallel()
	if got := langtag.Intelligible.Tier(); got != langtag.TierIntelligible {
		t.Errorf("Intelligible.Tier() = %v, want %v", got, langtag.TierIntelligible)
	}
	if got := langtag.SharedLiteracy.Tier(); got != langtag.TierSharedLiteracy {
		t.Errorf("SharedLiteracy.Tier() = %v, want %v", got, langtag.TierSharedLiteracy)
	}
	if got := langtag.Intelligible.String(); got != "intelligible" {
		t.Errorf("Intelligible.String() = %q, want %q", got, "intelligible")
	}
	if got := langtag.SharedLiteracy.String(); got != "shared-literacy" {
		t.Errorf("SharedLiteracy.String() = %q, want %q", got, "shared-literacy")
	}
}

// TestExcludedFamilyStaysOutAtEveryFloor extends the shared-literacy exclusion
// to the loosest floor a caller can set. The 33 pairs in
// TestCompareExcludesSharedLiteracy are absent from the table, so no opt-in can
// reach them; this asserts that rather than assuming it.
func TestExcludedFamilyStaysOutAtEveryFloor(t *testing.T) {
	t.Parallel()
	pairs := [][2]string{
		{"ta", "en"},
		{"eu", "es"},
		{"gl", "es"},
		{"cy", "en"},
		{"be", "ru"},
		{"ku", "tr"},
		{"lb", "de"},
		{"br", "fr"},
		{"jv", "id"},
		{"gu", "hi"},
		{"af", "nl"},
		{"mk", "bg"},
		{"et", "fi"},
		{"ms", "id"},
		{"sr", "hr"},
	}
	for _, floor := range []langtag.Tier{
		langtag.TierIdentical, langtag.TierSameLanguage, langtag.TierOtherScript,
		langtag.TierIntelligible, langtag.TierSharedLiteracy,
	} {
		for _, p := range pairs {
			a, b := langtag.MustParse(p[0]), langtag.MustParse(p[1])
			if langtag.Match(a, b, floor) {
				t.Errorf("Match(%q, %q, %v) = true, want false; this pair is not in the table at any tier",
					p[0], p[1], floor)
			}
		}
	}
}
