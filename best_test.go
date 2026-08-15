package langtag_test

import (
	"slices"
	"testing"

	"github.com/cplieger/langtag"
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
			floor:  langtag.TierSensitive,
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
			floor:  langtag.TierSensitive,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
		"floor excludes an otherwise-valid candidate": {
			want: "nob", available: []string{"nno"},
			floor:  langtag.TierOtherScript,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
		"empty candidate list": {
			want: "en", available: nil,
			floor:  langtag.TierSensitive,
			picked: nil, tier: langtag.TierNone, ok: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, tier, ok := langtag.Best(
				langtag.MustParse(tc.want), tracks(t, tc.available...), trackLang, tc.floor)
			if ok != tc.ok {
				t.Fatalf("Best(%q, %v, %v) ok = %v, want %v", tc.want, tc.available, tc.floor, ok, tc.ok)
			}
			if tier != tc.tier {
				t.Errorf("Best(%q, %v, %v) tier = %v, want %v", tc.want, tc.available, tc.floor, tier, tc.tier)
			}
			if gotNames := names(got); !slices.Equal(gotNames, tc.picked) {
				t.Errorf("Best(%q, %v, %v) picked %v, want %v",
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
	got, tier, ok := langtag.Best(langtag.MustParse("nob"), candidates, trackLang, langtag.TierSameLanguage)
	if !ok {
		t.Fatal("Best(nob, [untagged nor], same-language) ok = false, want true")
	}
	if tier != langtag.TierSameLanguage {
		t.Errorf("Best(nob, [untagged nor], same-language) tier = %v, want %v", tier, langtag.TierSameLanguage)
	}
	if gotNames := names(got); !slices.Equal(gotNames, []string{"nor"}) {
		t.Errorf("Best(nob, [untagged nor], same-language) picked %v, want [nor]", gotNames)
	}
}

// TestBestCapsFloorAtSensitive pins that TierNone cannot be used as a floor.
// Left uncapped it would accept every language as a stand-in for every other,
// which is never what a caller means by "be permissive".
func TestBestCapsFloorAtSensitive(t *testing.T) {
	t.Parallel()
	got, tier, ok := langtag.Best(
		langtag.MustParse("nob"), tracks(t, "swe", "eng"), trackLang, langtag.TierNone)
	if ok {
		t.Errorf("Best(nob, [swe eng], none) = (%v, %v, true), want ok=false", names(got), tier)
	}
}

func TestWithFallbacksNilDisablesTierThree(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks(nil)
	nob, nn := langtag.MustParse("nob"), langtag.MustParse("nn")
	if got := c.Compare(nob, nn); got != langtag.TierNone {
		t.Errorf("WithFallbacks(nil).Compare(nob, nn) = %v, want %v", got, langtag.TierNone)
	}
	// Tiers 0 to 2 are structural and must be unaffected by the table.
	if got := c.Compare(nob, langtag.MustParse("nor")); got != langtag.TierSameLanguage {
		t.Errorf("WithFallbacks(nil).Compare(nob, nor) = %v, want %v", got, langtag.TierSameLanguage)
	}
}

func TestWithFallbacksCustomTable(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "sv", Have: "no", Reason: "test-only claim", Both: false},
	})
	sv, no := langtag.MustParse("sv"), langtag.MustParse("no")
	if got := c.Compare(sv, no); got != langtag.TierSensitive {
		t.Errorf("custom Compare(sv, no) = %v, want %v", got, langtag.TierSensitive)
	}
	if got := c.Compare(no, sv); got != langtag.TierNone {
		t.Errorf("custom Compare(no, sv) = %v, want %v (entry is one-way)", got, langtag.TierNone)
	}
	// A custom table replaces the built-in one rather than extending it.
	if got := c.Compare(langtag.MustParse("nb"), langtag.MustParse("nn")); got != langtag.TierNone {
		t.Errorf("custom Compare(nb, nn) = %v, want %v (built-in entries must not leak in)", got, langtag.TierNone)
	}
}

// TestWithFallbacksIgnoresDegenerateEntries covers a table author's mistakes:
// an entry naming one language on both sides, or a blank side, cannot change any
// answer and must not corrupt the lookup.
func TestWithFallbacksIgnoresDegenerateEntries(t *testing.T) {
	t.Parallel()
	c := langtag.WithFallbacks([]langtag.Fallback{
		{Want: "en", Have: "en", Reason: "self"},
		{Want: "", Have: "es", Reason: "blank want"},
		{Want: "ca", Have: "", Reason: "blank have"},
	})
	if got := c.Compare(langtag.MustParse("en"), langtag.MustParse("en")); got != langtag.TierIdentical {
		t.Errorf("Compare(en, en) = %v, want %v", got, langtag.TierIdentical)
	}
	if got := c.Compare(langtag.MustParse("ca"), langtag.MustParse("es")); got != langtag.TierNone {
		t.Errorf("Compare(ca, es) with only degenerate entries = %v, want %v", got, langtag.TierNone)
	}
}

func TestReason(t *testing.T) {
	t.Parallel()
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	got, ok := langtag.Reason(ca, es)
	if !ok {
		t.Fatal("Reason(ca, es) ok = false, want true")
	}
	if got == "" {
		t.Error("Reason(ca, es) = \"\", want a justification")
	}
	if _, ok := langtag.Reason(es, ca); ok {
		t.Error("Reason(es, ca) ok = true, want false (the entry is one-way)")
	}
	if _, ok := langtag.Reason(langtag.MustParse("nob"), langtag.MustParse("nor")); ok {
		t.Error("Reason(nob, nor) ok = true, want false (tier 1 is structural, not a judgment)")
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
		"identical":          {"identical", langtag.TierIdentical, true},
		"same-language":      {"same-language", langtag.TierSameLanguage, true},
		"underscore form":    {"same_language", langtag.TierSameLanguage, true},
		"uppercase":          {"OTHER-SCRIPT", langtag.TierOtherScript, true},
		"mixed with spaces":  {" Sensitive ", langtag.TierSensitive, true},
		"none is not a knob": {"none", langtag.TierNone, false},
		"unknown":            {"loose", langtag.TierNone, false},
		"empty":              {"", langtag.TierNone, false},
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
		langtag.TierOtherScript, langtag.TierSensitive,
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
	if got := langtag.Tier(99).String(); got != "invalid" {
		t.Errorf("Tier(99).String() = %q, want %q", got, "invalid")
	}
}
