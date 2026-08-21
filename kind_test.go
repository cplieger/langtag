package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v2"
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
		if langtag.Prefer(want).Match(have, langtag.TierIntelligible) {
			t.Errorf("Prefer(%q).Match(%q, intelligible) = true, want false; %q is a shared-literacy entry and must need its own opt-in",
				f.Want, f.Have, f.Want+"->"+f.Have)
		}
		if !langtag.Prefer(want).Match(have, langtag.TierSharedLiteracy) {
			t.Errorf("Prefer(%q).Match(%q, shared-literacy) = false, want true", f.Want, f.Have)
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
	if got := c.Prefer(ca).Compare(es); got != langtag.TierSharedLiteracy {
		t.Errorf("Prefer(ca).Compare(es) = %v, want %v", got, langtag.TierSharedLiteracy)
	}
	if got := c.Prefer(es).Compare(ca); got != langtag.TierNone {
		t.Errorf("Prefer(es).Compare(ca) = %v, want %v (a shared-literacy claim is not reciprocated)",
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
			if got := c.Prefer(sv).Compare(no); got != langtag.TierNone {
				t.Errorf("Prefer(sv).Compare(no) with a malformed entry = %v, want %v", got, langtag.TierNone)
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
		if langtag.Prefer(ta).Match(en, floor) {
			t.Errorf("Prefer(ta).Match(en, %v) = true, want false", floor)
		}
		if langtag.Prefer(zero).Match(zero, floor) {
			t.Errorf("Prefer(zero).Match(zero, %v) = true, want false", floor)
		}
		got, tier, ok := langtag.Prefer(langtag.MustParse("ca")).Best(
			[]langtag.Tag{langtag.MustParse("es")},
			func(t langtag.Tag) langtag.Tag { return t }, floor,
		)
		if ok {
			t.Errorf("Best(Prefer(ca), [es], %v) = (%v, %v, true), want ok=false; an unusable floor must not widen to the most permissive tier",
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
	if langtag.Prefer(langtag.MustParse("ta")).Match(langtag.MustParse("en"), floor) {
		t.Error("a caller that ignored ParseTier's ok value got a floor that matches everything, want one that matches nothing")
	}
}

// TestZeroPreferenceFailsClosed pins the zero value's documented contract. A
// Preference nobody constructed prefers the zero Tag, so it must answer
// TierNone everywhere, explain nothing, and select nothing — without
// panicking, because every method checks the nil table before touching it
// (the zero Tag short-circuit alone would also refuse, but the nil check is
// what makes the refusal hold for a REAL want bound to a nil table below).
func TestZeroPreferenceFailsClosed(t *testing.T) {
	t.Parallel()
	var p langtag.Preference
	en := langtag.MustParse("en")
	if got := p.Compare(en); got != langtag.TierNone {
		t.Errorf("zero Preference: Compare(en) = %v, want %v", got, langtag.TierNone)
	}
	if _, ok := p.Reason(en); ok {
		t.Error("zero Preference: Reason(en) ok = true, want false")
	}
	if p.Match(en, langtag.TierSharedLiteracy) {
		t.Error("zero Preference: Match(en, shared-literacy) = true, want false")
	}
	got, tier, ok := p.Best([]langtag.Tag{en},
		func(t langtag.Tag) langtag.Tag { return t }, langtag.TierSharedLiteracy)
	if ok {
		t.Errorf("Best(zero Preference, [en], shared-literacy) = (%v, %v, true), want ok=false", got, tier)
	}
}

// A nil *Comparer must fail CLOSED: a Preference bound to a nil table judges
// nothing acceptable, rather than silently inheriting the built-in table.
// Inheriting would be a widening — the exact direction every fail-open bug in
// this library's history took — triggered by nothing louder than a nil field.
func TestNilComparerPreferenceFailsClosed(t *testing.T) {
	t.Parallel()
	var c *langtag.Comparer
	p := c.Prefer(langtag.MustParse("nb")) // a REAL want; only the table is nil
	nn := langtag.MustParse("nn")
	if got := p.Compare(nn); got != langtag.TierNone {
		t.Errorf("nil-Comparer Preference: Compare(nn) = %v, want %v (must not inherit the built-in table, which grades nb/nn %v)",
			got, langtag.TierNone, langtag.TierIntelligible)
	}
	if _, ok := p.Reason(nn); ok {
		t.Error("nil-Comparer Preference: Reason(nn) ok = true, want false")
	}
	if p.Match(nn, langtag.TierIntelligible) {
		t.Error("nil-Comparer Preference: Match(nn, intelligible) = true, want false")
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
	// An unset or unknown Kind lands on the tier that licenses nothing. This is
	// the whole reason Kind has no usable zero value: a malformed entry that
	// reached the tier scale at all would inherit the stronger of the two
	// curated claims, which is the error the Kind/direction split exists to
	// prevent.
	if got := langtag.KindUnset.Tier(); got != langtag.TierNone {
		t.Errorf("KindUnset.Tier() = %v, want %v", got, langtag.TierNone)
	}
	if got := langtag.Kind(99).Tier(); got != langtag.TierNone {
		t.Errorf("Kind(99).Tier() = %v, want %v", got, langtag.TierNone)
	}
	if got := langtag.KindUnset.String(); got != "unset" {
		t.Errorf("KindUnset.String() = %q, want %q", got, "unset")
	}
	if got := langtag.Kind(99).String(); got != "unset" {
		t.Errorf("Kind(99).String() = %q, want %q", got, "unset")
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
			if langtag.Prefer(a).Match(b, floor) {
				t.Errorf("Prefer(%q).Match(%q, %v) = true, want false; this pair is not in the table at any tier",
					p[0], p[1], floor)
			}
		}
	}
}

// TestConflictingEntriesKeepTheFartherTier pins the order-dependence a diff
// review found.
//
// Two entries can name one ordered pair, most easily when a symmetric entry's
// reciprocal direction collides with an explicit one-way entry for the same
// pair. Overwriting would let table order decide how close a pair is, and would
// silently promote a declared shared-literacy claim onto the intelligible tier.
// The farther tier wins, so a later entry can only ever narrow what a floor
// accepts, and ValidateFallbacks names the conflict rather than hiding it.
func TestConflictingEntriesKeepTheFartherTier(t *testing.T) {
	t.Parallel()
	table := []langtag.Fallback{
		{Want: "ca", Have: "es", Kind: langtag.SharedLiteracy, Reason: "one-way claim"},
		{Want: "es", Have: "ca", Kind: langtag.Intelligible, Both: true, Reason: "symmetric claim"},
	}
	for _, order := range [][]langtag.Fallback{table, {table[1], table[0]}} {
		c := langtag.WithFallbacks(order)
		ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
		if got := c.Prefer(ca).Compare(es); got != langtag.TierSharedLiteracy {
			t.Errorf("Prefer(ca).Compare(es) = %v, want %v; the farther tier must win regardless of table order",
				got, langtag.TierSharedLiteracy)
		}
	}
	errs := langtag.ValidateFallbacks(table)
	if len(errs) == 0 {
		t.Error("ValidateFallbacks reported no conflict for a table claiming one pair at two tiers, want one")
	}
}

// TestComparerMatchHonorsTheFloorContract covers the gap a diff review found: a
// caller using a custom table reached Compare but had no Match, so it had to
// re-implement the floor comparison and would re-inherit the fail-open bug that
// the built-in-table Match had already been fixed for.
func TestComparerMatchHonorsTheFloorContract(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks(langtag.Fallbacks())
	nob, nn := langtag.MustParse("nob"), langtag.MustParse("nn")
	ta, en := langtag.MustParse("ta"), langtag.MustParse("en")

	if !c.Prefer(nob).Match(nn, langtag.TierIntelligible) {
		t.Error("Comparer.Prefer(nob).Match(nn, intelligible) = false, want true")
	}
	if c.Prefer(nob).Match(nn, langtag.TierOtherScript) {
		t.Error("Comparer.Prefer(nob).Match(nn, other-script) = true, want false")
	}
	for _, floor := range []langtag.Tier{langtag.TierNone, langtag.Tier(99)} {
		if c.Prefer(ta).Match(en, floor) {
			t.Errorf("Comparer.Prefer(ta).Match(en, %v) = true, want false; an unusable floor must match nothing", floor)
		}
	}
}
