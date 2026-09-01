package langtag_test

import (
	"fmt"
	"testing"

	"github.com/cplieger/langtag/v2"
)

// Compare and Parse are measured separately because Parse costs one to two
// orders of magnitude more (270-440 ns against 4-25 ns on one developer
// machine), so a combined number would hide a regression in the tier ladder
// inside parse noise. BenchmarkPreferenceAcrossTracks charts the shape a
// consumer actually runs: one Preference judging many tracks, reused rather
// than rebuilt per candidate.
//
// TestPreferenceCompareIsAllocationFree and
// TestBestDoesNotAllocateWhenNothingMatches gate allocation counts that were
// measured to hold before being asserted; the benchmarks below feed the
// tracker, one series per distinct path through the ladder so a regression in
// one path is not averaged away by the others. Neither allocation contract
// calls t.Parallel: testing.AllocsPerRun pins GOMAXPROCS to 1 for its
// measurement window, so a concurrent sibling's allocations would be
// misattributed.
//
// Compare does not walk the fallback table — it is two map lookups
// (compare.go) — so BenchmarkCompareFallbackBucket's two series must stay
// flat regardless of table size; that is the property it charts.

// compareSink* consume every result these benchmarks produce. b.Loop keeps the
// call itself from being elided, but a return value nothing reads is still
// eliminable, and a benchmark measuring an elided call charts a constant forever.
var (
	compareSinkTier langtag.Tier
	compareSinkBool bool
	compareSinkStr  string
	compareSinkTag  langtag.Tag
	compareSinkTags []langtag.Tag
	compareSinkPref langtag.Preference
)

// tierPair is one wanted/available pair together with the tier it must grade
// to, so a fixture drift onto another path is caught before it is measured.
type tierPair struct {
	name string
	want string
	have string
	tier langtag.Tier
	// charted marks a pair with a distinct cost path; every pair is still
	// checked for allocations regardless.
	charted bool
}

// tierPairs covers every tier the ladder defines. same_language_close_script
// is the only linear scan in the comparison path (scriptsReadAsOne in
// scripts.go) and is charted separately from the macro/region same-language
// cases, which share a branch. shared_literacy is uncharted because it
// reaches the same map lookup as intelligible; compare_test.go covers its
// correctness.
func tierPairs() []tierPair {
	return []tierPair{
		{"identical", "ger", "deu", langtag.TierIdentical, true},
		{"same_language_macro", "nob", "nor", langtag.TierSameLanguage, true},
		{"same_language_region", "es-ES", "es-419", langtag.TierSameLanguage, false},
		{"same_language_close_script", "sr-Latn", "sr-Cyrl", langtag.TierSameLanguage, true},
		{"other_script", "zh-Hans", "zh-Hant", langtag.TierOtherScript, true},
		{"intelligible", "nb", "nn", langtag.TierIntelligible, true},
		{"shared_literacy", "ca", "es", langtag.TierSharedLiteracy, false},
		{"none", "no", "sv", langtag.TierNone, true},
	}
}

// TestPreferenceCompareIsAllocationFree pins that every exported method a
// comparison site calls costs nothing on the heap: an allocation here is
// multiplied by tracks times episodes. The zero-tag case is separate because
// compare.go returns TierNone before touching the table, so it must stay the
// cheapest comparison rather than becoming the most expensive.
func TestPreferenceCompareIsAllocationFree(t *testing.T) {
	for _, pair := range tierPairs() {
		t.Run(pair.name, func(t *testing.T) {
			want := langtag.MustParse(pair.want)
			have := langtag.MustParse(pair.have)
			pref := langtag.Prefer(want)

			// A drifted fixture would measure the wrong path under this name, so
			// establish which path runs before measuring what it costs.
			if got := pref.Compare(have); got != pair.tier {
				t.Fatalf("Prefer(%q).Compare(%q) = %v, want %v: the fixture no longer "+
					"exercises the path this case is named for",
					pair.want, pair.have, got, pair.tier)
			}

			calls := map[string]func(){
				"Compare": func() { compareSinkTier = pref.Compare(have) },
				"Match":   func() { compareSinkBool = pref.Match(have, langtag.TierIntelligible) },
				"Reason":  func() { compareSinkStr, compareSinkBool = pref.Reason(have) },
				"Prefer":  func() { compareSinkPref = langtag.Prefer(want) },
				"Want":    func() { compareSinkTag = pref.Want() },
				"String":  func() { compareSinkStr = pref.String() },
			}
			for method, call := range calls {
				if got := testing.AllocsPerRun(100, call); got != 0 {
					t.Errorf("Preference.%s on %q -> %q (%v) allocated %v times per run, want 0",
						method, pair.want, pair.have, pair.tier, got)
				}
			}
		})
	}

	t.Run("zero_tag", func(t *testing.T) {
		pref := langtag.Prefer(langtag.Tag{})
		have := langtag.MustParse("en")
		if got := pref.Compare(have); got != langtag.TierNone {
			t.Fatalf("Prefer(zero Tag).Compare(%q) = %v, want %v", "en", got, langtag.TierNone)
		}
		if got := testing.AllocsPerRun(100, func() {
			compareSinkTier = pref.Compare(have)
		}); got != 0 {
			t.Errorf("Preference.Compare on a zero wanted Tag allocated %v times per run, want 0",
				got)
		}
	})
}

// TestBestDoesNotAllocateWhenNothingMatches pins the fail-closed path's cost:
// a library of tracks tagged in languages nobody asked for hits this path
// most often, and compare.go leaves the output slice nil until a candidate
// lands within the floor, so it must cost only the scan.
func TestBestDoesNotAllocateWhenNothingMatches(t *testing.T) {
	const floor = langtag.TierIntelligible

	pref := langtag.Prefer(langtag.MustParse("nob"))
	tagOf := func(tag langtag.Tag) langtag.Tag { return tag }
	candidates := benchTracks(16, "ja", "ko", "th", "he", "ar", "fa")

	if _, _, ok := pref.Best(candidates, tagOf, floor); ok {
		t.Fatalf("Prefer(%q).Best(%d candidates, floor %v) matched something; the fixture "+
			"must hold nothing within the floor",
			"nob", len(candidates), floor)
	}
	if got := testing.AllocsPerRun(100, func() {
		compareSinkTags, compareSinkTier, compareSinkBool = pref.Best(candidates, tagOf, floor)
	}); got != 0 {
		t.Errorf("Preference.Best over %d non-matching candidates allocated %v times per run, "+
			"want 0: the result slice must stay nil until something is within the floor",
			len(candidates), got)
	}
}

// BenchmarkCompare measures each distinct path through the tier ladder. This is
// the first-party half of the library's cost, and it is charted per path because
// the paths do different work: an identical answer is one string comparison, a
// cross-language answer is two map lookups, and a close-script answer walks the
// CloseScripts list.
func BenchmarkCompare(b *testing.B) {
	for _, pair := range tierPairs() {
		if !pair.charted {
			continue
		}
		want := langtag.MustParse(pair.want)
		have := langtag.MustParse(pair.have)
		pref := langtag.Prefer(want)
		b.Run(pair.name, func(b *testing.B) {
			if got := pref.Compare(have); got != pair.tier {
				b.Fatalf("Prefer(%q).Compare(%q) = %v, want %v: the fixture no longer "+
					"exercises the path this series is named for",
					pair.want, pair.have, got, pair.tier)
			}
			b.ReportAllocs()
			for b.Loop() {
				compareSinkTier = pref.Compare(have)
			}
		})
	}
}

// BenchmarkCompareFallbackBucket charts one lookup against buckets 256 times
// apart in size, to catch a future map-to-slice change: compare reads the
// wanted language's bucket with one map index and the available language out
// of it with a second, so these two series must stay flat regardless of
// table size.
func BenchmarkCompareFallbackBucket(b *testing.B) {
	want := langtag.MustParse("no")
	have := langtag.MustParse("sv")
	for _, n := range []int{8, 2048} {
		comparer := langtag.WithFallbacks(benchFallbackTable(n))
		pref := comparer.Prefer(want)
		b.Run(fmt.Sprintf("bucket_%d", n), func(b *testing.B) {
			if got := pref.Compare(have); got != langtag.TierIntelligible {
				b.Fatalf("Prefer(%q).Compare(%q) against a %d-entry table = %v, want %v: the "+
					"synthetic table no longer resolves, so this series would chart a miss",
					"no", "sv", n, got, langtag.TierIntelligible)
			}
			b.ReportAllocs()
			for b.Loop() {
				compareSinkTier = pref.Compare(have)
			}
		})
	}
}

// BenchmarkPreferenceAcrossTracks charts the shape a consumer actually runs:
// one wanted language judged against every track an episode offers. reuse
// builds the Preference once outside the track loop (the idiom both
// consumers use); rebuild constructs one per candidate, charted at 64 tracks
// only — rebuilding costs 1-2 ns per candidate, inside this machine's
// run-to-run spread at 8.
func BenchmarkPreferenceAcrossTracks(b *testing.B) {
	// A plausible mixed-language episode: a few tracks that grade at various
	// tiers and a majority that do not match at all.
	langs := []string{"eng", "nor", "swe", "dan", "nno", "fra", "deu", "jpn"}
	want := langtag.MustParse("nob")

	for _, tracks := range []int{8, 64} {
		candidates := benchTracks(tracks, langs...)
		b.Run(fmt.Sprintf("reuse_tracks_%d", tracks), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				pref := langtag.Prefer(want)
				for _, have := range candidates {
					compareSinkTier = pref.Compare(have)
				}
			}
		})
	}

	candidates := benchTracks(64, langs...)
	b.Run("rebuild_tracks_64", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, have := range candidates {
				compareSinkTier = langtag.Prefer(want).Compare(have)
			}
		}
	})
}

// benchTracks builds n parsed tags by cycling through langs, parsing during
// setup so no benchmark pays parse cost inside its timed loop.
func benchTracks(n int, langs ...string) []langtag.Tag {
	out := make([]langtag.Tag, 0, n)
	for i := range n {
		out = append(out, langtag.MustParse(langs[i%len(langs)]))
	}
	return out
}

// benchFallbackTable builds a table of n entries all naming "no" as the
// wanted language, so they land in one bucket and a scan would be visible.
// The stand-in Have values are synthetic and never match a real tag, so the
// only reachable entry is the last: "no" -> "sv".
func benchFallbackTable(n int) []langtag.Fallback {
	const (
		letters = "abcdefghijklmnopqrstuvwxyz"
		reason  = "synthetic benchmark entry, not a claim about any language"
		source  = "none; this table exists only to give a lookup a size"
	)
	out := make([]langtag.Fallback, 0, n)
	for i := range n - 1 {
		have := string([]byte{
			letters[i%26],
			letters[(i/26)%26],
			letters[(i/676)%26],
		})
		out = append(out, langtag.Fallback{
			Want: "no", Have: have, Kind: langtag.Intelligible, Both: true,
			Reason: reason, Provenance: source,
		})
	}
	return append(out, langtag.Fallback{
		Want: "no", Have: "sv", Kind: langtag.Intelligible, Both: true,
		Reason: reason, Provenance: source,
	})
}
