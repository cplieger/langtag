package langtag_test

import (
	"slices"
	"testing"

	"github.com/cplieger/langtag/v2"
)

// track models a caller's own candidate type, to exercise that Best works
// against arbitrary values rather than a []Tag the caller has to build.
type track struct {
	name string
	lang langtag.Tag
}

func tracks(t *testing.T, specs ...string) []track {
	t.Helper()
	out := make([]track, 0, len(specs))
	for _, s := range specs {
		out = append(out, track{name: s, lang: langtag.MustParse(s)})
	}
	return out
}

func names(ts []track) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.name)
	}
	return out
}

func trackLang(t track) langtag.Tag { return t.lang }

func TestBest(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want      string
		available []string
		floor     langtag.Tier
		picked    []string
		tier      langtag.Tier
		ok        bool
	}{
		"the reported bug: bokmal reference, norwegian-only episode": {
			want: "nob", available: []string{"eng", "nor"},
			floor:  langtag.TierSameLanguage,
			picked: []string{"nor"}, tier: langtag.TierSameLanguage, ok: true,
		},
		"exact still wins when both variants are present": {
			want: "nob", available: []string{"eng", "nor", "nob", "nno"},
			floor:  langtag.TierIntelligible,
			picked: []string{"nob"}, tier: langtag.TierIdentical, ok: true,
		},
		"the spanish pair: es-ES reference prefers es-ES over es-419": {
			want: "es-ES", available: []string{"es-419", "es-ES", "eng"},
			floor:  langtag.TierSameLanguage,
			picked: []string{"es-ES"}, tier: langtag.TierIdentical, ok: true,
		},
		"every same-tier candidate is returned for the caller to rank": {
			want: "en", available: []string{"en-GB", "en-US", "en-AU"},
			floor:  langtag.TierSameLanguage,
			picked: []string{"en-GB", "en-US", "en-AU"}, tier: langtag.TierSameLanguage, ok: true,
		},
		"input order is preserved": {
			want: "es", available: []string{"es-419", "es-MX", "es-ES"},
			floor:  langtag.TierSameLanguage,
			picked: []string{"es-419", "es-MX", "es-ES"}, tier: langtag.TierSameLanguage, ok: true,
		},
		"a closer candidate discards the farther ones already collected": {
			want: "zh-Hans", available: []string{"zh-Hant", "zh-Hans"},
			floor:  langtag.TierOtherScript,
			picked: []string{"zh-Hans"}, tier: langtag.TierIdentical, ok: true,
		},
		"nothing within floor": {
			want: "nob", available: []string{"eng", "swe"},
			floor:  langtag.TierIntelligible,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
		"floor excludes an otherwise-valid candidate": {
			want: "nob", available: []string{"nno"},
			floor:  langtag.TierOtherScript,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
		"empty candidate list": {
			want: "en", available: nil,
			floor:  langtag.TierIntelligible,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, tier, ok := langtag.Prefer(langtag.MustParse(tc.want)).Best(
				tracks(t, tc.available...), trackLang, tc.floor,
			)
			if ok != tc.ok {
				t.Fatalf("Best(Prefer(%q), %v, %v) ok = %v, want %v", tc.want, tc.available, tc.floor, ok, tc.ok)
			}
			if tier != tc.tier {
				t.Errorf("Best(Prefer(%q), %v, %v) tier = %v, want %v", tc.want, tc.available, tc.floor, tier, tc.tier)
			}
			if gotNames := names(got); !slices.Equal(gotNames, tc.picked) {
				t.Errorf("Best(Prefer(%q), %v, %v) picked %v, want %v",
					tc.want, tc.available, tc.floor, gotNames, tc.picked)
			}
		})
	}
}

// TestBestSkipsUnparsedCandidates covers the realistic case of an episode
// carrying a track Plex could not label. It must never be chosen, and it must
// not prevent a real candidate from being found.
func TestBestSkipsUnparsedCandidates(t *testing.T) {
	t.Parallel()
	candidates := []track{
		{name: "untagged", lang: langtag.Tag{}},
		{name: "nor", lang: langtag.MustParse("nor")},
	}
	got, tier, ok := langtag.Prefer(langtag.MustParse("nob")).Best(candidates, trackLang, langtag.TierSameLanguage)
	if !ok {
		t.Fatal("Best(Prefer(nob), [untagged nor], same-language) ok = false, want true")
	}
	if tier != langtag.TierSameLanguage {
		t.Errorf("Best(Prefer(nob), [untagged nor], same-language) tier = %v, want %v", tier, langtag.TierSameLanguage)
	}
	if gotNames := names(got); !slices.Equal(gotNames, []string{"nor"}) {
		t.Errorf("Best(Prefer(nob), [untagged nor], same-language) picked %v, want [nor]", gotNames)
	}
}

// TestBestCapsFloorAtSensitive pins that TierNone cannot be used as a floor.
// Left uncapped it would accept every language as a stand-in for every other,
// which is never what a caller means by "be permissive".
func TestBestCapsFloorAtSensitive(t *testing.T) {
	t.Parallel()
	got, tier, ok := langtag.Prefer(langtag.MustParse("nob")).Best(
		tracks(t, "swe", "eng"), trackLang, langtag.TierNone,
	)
	if ok {
		t.Errorf("Best(Prefer(nob), [swe eng], none) = (%v, %v, true), want ok=false", names(got), tier)
	}
}

func TestWithFallbacksNilDisablesTierThree(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks(nil)
	nob, nn := langtag.MustParse("nob"), langtag.MustParse("nn")
	if got := c.Prefer(nob).Compare(nn); got != langtag.TierNone {
		t.Errorf("WithFallbacks(nil).Prefer(nob).Compare(nn) = %v, want %v", got, langtag.TierNone)
	}
	// Tiers 0 to 2 are structural and must be unaffected by the table.
	if got := c.Prefer(nob).Compare(langtag.MustParse("nor")); got != langtag.TierSameLanguage {
		t.Errorf("WithFallbacks(nil).Prefer(nob).Compare(nor) = %v, want %v", got, langtag.TierSameLanguage)
	}
}

func TestWithFallbacksCustomTable(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "sv", Have: "no", Reason: "test-only claim", Kind: langtag.Intelligible, Both: true},
	})
	sv, no := langtag.MustParse("sv"), langtag.MustParse("no")
	if got := c.Prefer(sv).Compare(no); got != langtag.TierIntelligible {
		t.Errorf("custom Prefer(sv).Compare(no) = %v, want %v", got, langtag.TierIntelligible)
	}
	if got := c.Prefer(no).Compare(sv); got != langtag.TierIntelligible {
		t.Errorf("custom Prefer(no).Compare(sv) = %v, want %v (a symmetric entry runs both ways)", got, langtag.TierIntelligible)
	}
	// A custom table replaces the built-in one rather than extending it.
	if got := c.Prefer(langtag.MustParse("nb")).Compare(langtag.MustParse("nn")); got != langtag.TierNone {
		t.Errorf("custom Prefer(nb).Compare(nn) = %v, want %v (built-in entries must not leak in)", got, langtag.TierNone)
	}
}

// TestWithFallbacksIgnoresDegenerateEntries covers a table author's mistakes:
// an entry naming one language on both sides, or a blank side, cannot change any
// answer and must not corrupt the lookup.
func TestWithFallbacksIgnoresDegenerateEntries(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "en", Have: "en", Reason: "self", Kind: langtag.Intelligible, Both: true},
		{Want: "", Have: "es", Reason: "blank want", Kind: langtag.Intelligible, Both: true},
		{Want: "ca", Have: "", Reason: "blank have", Kind: langtag.SharedLiteracy},
	})
	if got := c.Prefer(langtag.MustParse("en")).Compare(langtag.MustParse("en")); got != langtag.TierIdentical {
		t.Errorf("Prefer(en).Compare(en) = %v, want %v", got, langtag.TierIdentical)
	}
	if got := c.Prefer(langtag.MustParse("ca")).Compare(langtag.MustParse("es")); got != langtag.TierNone {
		t.Errorf("Prefer(ca).Compare(es) with only degenerate entries = %v, want %v", got, langtag.TierNone)
	}
}

func TestReason(t *testing.T) {
	t.Parallel()
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	got, ok := langtag.Prefer(ca).Reason(es)
	if !ok {
		t.Fatal("Prefer(ca).Reason(es) ok = false, want true")
	}
	if got == "" {
		t.Error("Prefer(ca).Reason(es) = \"\", want a justification")
	}
	if _, ok := langtag.Prefer(es).Reason(ca); ok {
		t.Error("Prefer(es).Reason(ca) ok = true, want false (the entry is one-way)")
	}
	if _, ok := langtag.Prefer(langtag.MustParse("nob")).Reason(langtag.MustParse("nor")); ok {
		t.Error("Prefer(nob).Reason(nor) ok = true, want false (tier 1 is structural, not a judgment)")
	}
}

// TestFallbacksTable pins the shipped judgments. Each entry is a claim about
// people, so a change here is a decision and should show up as a failing test
// rather than as a quiet diff.
func TestFallbacksTable(t *testing.T) {
	t.Parallel()
	type entry struct {
		want, have string
		both       bool
	}
	wanted := []entry{
		{"no", "nn", true},
		{"no", "da", true},
		{"hr", "bs", true},
		{"cs", "sk", true},
		{"ca", "es", false},
	}
	got := langtag.Fallbacks()
	if len(got) != len(wanted) {
		t.Fatalf("Fallbacks() returned %d entries, want %d: %+v", len(got), len(wanted), got)
	}
	for i, w := range wanted {
		g := got[i]
		if g.Want != w.want || g.Have != w.have || g.Both != w.both {
			t.Errorf("Fallbacks()[%d] = {%s %s both=%v}, want {%s %s both=%v}",
				i, g.Want, g.Have, g.Both, w.want, w.have, w.both)
		}
		if g.Reason == "" {
			t.Errorf("Fallbacks()[%d] (%s -> %s) has an empty Reason; every entry must carry its argument",
				i, g.Want, g.Have)
		}
	}
}

// TestFallbacksReturnsACopy prevents a caller from editing the shipped table
// through the accessor.
func TestFallbacksReturnsACopy(t *testing.T) {
	t.Parallel()
	first := langtag.Fallbacks()
	if len(first) == 0 {
		t.Fatal("Fallbacks() returned no entries")
	}
	original := first[0]
	first[0] = langtag.Fallback{Want: "xx", Have: "yy", Reason: "mutated"}
	second := langtag.Fallbacks()
	if second[0] != original {
		t.Errorf("Fallbacks()[0] = %+v after a caller mutated an earlier result, want %+v",
			second[0], original)
	}
}

// TestFallbackEntriesUseLanguageKeys guards the one way a table entry silently
// does nothing: naming a language by a spelling that Tag.Language never
// produces, such as "nb" where folding yields "no".
func TestFallbackEntriesUseLanguageKeys(t *testing.T) {
	t.Parallel()
	for _, f := range langtag.Fallbacks() {
		for _, side := range []string{f.Want, f.Have} {
			tag, ok := langtag.Parse(side)
			if !ok {
				t.Errorf("fallback entry names %q, which is not a language tag", side)
				continue
			}
			if tag.Language() != side {
				t.Errorf("fallback entry names %q, but Parse(%q).Language() = %q; the entry would never match",
					side, side, tag.Language())
			}
		}
	}
}

func TestParseTier(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		tier langtag.Tier
		ok   bool
	}{
		"identical":           {"identical", langtag.TierIdentical, true},
		"same-language":       {"same-language", langtag.TierSameLanguage, true},
		"underscore form":     {"same_language", langtag.TierSameLanguage, true},
		"uppercase":           {"OTHER-SCRIPT", langtag.TierOtherScript, true},
		"mixed with spaces":   {" Intelligible ", langtag.TierIntelligible, true},
		"shared literacy":     {"shared-literacy", langtag.TierSharedLiteracy, true},
		"underscore literacy": {"shared_literacy", langtag.TierSharedLiteracy, true},
		"none is not a knob":  {"none", langtag.TierNone, false},
		"unknown":             {"loose", langtag.TierNone, false},
		"empty":               {"", langtag.TierNone, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tier, ok := langtag.ParseTier(tc.in)
			if ok != tc.ok || tier != tc.tier {
				t.Errorf("ParseTier(%q) = (%v, %v), want (%v, %v)", tc.in, tier, ok, tc.tier, tc.ok)
			}
		})
	}
}

// TestTierStringRoundTrips keeps the configuration surface and the parser from
// drifting apart, which is how a documented value stops being accepted.
func TestTierStringRoundTrips(t *testing.T) {
	t.Parallel()
	for _, tier := range []langtag.Tier{
		langtag.TierIdentical, langtag.TierSameLanguage,
		langtag.TierOtherScript, langtag.TierIntelligible, langtag.TierSharedLiteracy,
	} {
		s := tier.String()
		got, ok := langtag.ParseTier(s)
		if !ok {
			t.Errorf("ParseTier(%v.String() = %q) ok = false, want true", tier, s)
			continue
		}
		if got != tier {
			t.Errorf("ParseTier(%q) = %v, want %v", s, got, tier)
		}
	}
	// A tier past the last named one renders as "invalid" rather than indexing
	// off the end of the name table. TierNone+1 is the value that matters: it is
	// the first tier with no name, so it is the only input that distinguishes a
	// guard covering the whole table from one that leaves its last slot open.
	// Printed as a number, because %v on a Tier is the method under test.
	for _, tier := range []langtag.Tier{langtag.TierNone + 1, langtag.Tier(99)} {
		if got := tier.String(); got != "invalid" {
			t.Errorf("Tier(%d).String() = %q, want %q", uint8(tier), got, "invalid")
		}
	}
}
