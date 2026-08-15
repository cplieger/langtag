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

// TestSharedLiteracyIsNeverReciprocated covers the authoring mistake directly:
// even if a table sets Both on a shared-literacy entry, the reverse direction
// must not be admitted.
func TestSharedLiteracyIsNeverReciprocated(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "ca", Have: "es", Kind: langtag.SharedLiteracy, Both: true, Reason: "test"},
	})
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	if got := c.Compare(ca, es); got != langtag.TierSharedLiteracy {
		t.Errorf("Compare(ca, es) = %v, want %v", got, langtag.TierSharedLiteracy)
	}
	if got := c.Compare(es, ca); got != langtag.TierNone {
		t.Errorf("Compare(es, ca) = %v, want %v; Both must be ignored for a shared-literacy entry",
			got, langtag.TierNone)
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
