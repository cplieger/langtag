package langtag_test

import (
	"fmt"
	"testing"

	"github.com/cplieger/langtag/v2"
)

// This file exists because the tier ladder is the half of langtag that is
// entirely first-party policy, and it is the half a consumer runs in a loop. A
// regression here cannot be blamed on a dependency, and it cannot be seen in the
// parse numbers either: parse and compare are separate cost centres, measured
// separately here because measuring them together would let a slowdown in one
// hide behind the other. Parse costs one to two orders of magnitude more than a
// comparison (on one developer machine, 270-440 ns against 4-25 ns), so a
// combined number would be a parse number with rounding error where the tier
// policy used to be.
//
// The shape that actually multiplies is one Preference judging many tracks:
// plex-language-sync grades every available subtitle and audio track of every
// episode against one stored choice. BenchmarkPreferenceAcrossTracks charts that
// shape, and charts the rebuild-per-candidate variant beside it so the cost of
// the idiom a caller reaches for when it passes a Tag down instead of a
// Preference stays on the record rather than being re-argued.
//
// Two kinds of check, doing different jobs:
//
//   - TestPreferenceCompareIsAllocationFree and
//     TestBestDoesNotAllocateWhenNothingMatches GATE properties that were
//     MEASURED to hold before they were asserted. Comparison is the innermost
//     loop of the library, so an allocation introduced there is multiplied by
//     tracks times episodes; testing.AllocsPerRun is exact, so the assertions are
//     "== 0" and a future fmt call, string build or interface boxing in the
//     comparison path goes red at merge time rather than showing up later as a
//     chart that moved.
//   - The benchmarks feed the tracker. BenchmarkCompare charts one series per
//     distinct path through the ladder rather than one number for the whole of
//     it, because the paths do different amounts of work and a regression in one
//     would be invisible in an average over all of them.
//     BenchmarkCompareFallbackBucket is size-parameterised so a map lookup
//     turning into a scan shows as a super-linear jump between sizes rather than
//     a uniform slowdown that reads as runner noise.
//
// Neither contract below calls t.Parallel, deliberately: testing.AllocsPerRun
// measures a delta on a process-global allocation counter and pins GOMAXPROCS to
// 1 while it runs, so a concurrent sibling's allocations would be charged to
// whichever contract happened to be measuring.
//
// Three measurements taken while writing this, recorded because they decide what
// is charted and what is not. Absolute figures are from one developer machine and
// are quoted for their ratios, which is what survives a different runner:
//
//   - Compare does NOT walk the fallback table. It reads the wanted language's
//     bucket with a map index and the available language out of that bucket with
//     a second one (compare.go), so grading is O(1) in table size and only
//     WithFallbacks is linear in it. BenchmarkCompareFallbackBucket asserts that
//     division of labour by charting it: those series must stay flat.
//   - WithFallbacks is linear as intended (2.1 us, 34 us and 707 us for 8, 128
//     and 2048 entries: 16x the entries for 16x to 21x the time). It is not
//     charted because it runs once per process at startup, and the package's
//     benchmark budget buys more from a path a consumer runs per track.
//   - Preference.Best costs the Compare loop below plus one slice append per
//     candidate that ties at the best tier (296 ns and 1 allocation over 8
//     tracks, 2.4 us and 4 over 64). A series for it would be ~90% the same
//     measurement as reuse_tracks_N, so its cost rides those series and its
//     allocation behaviour is gated by the test instead.

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

// tierPair is one wanted/available pair together with the tier it must grade to.
//
// Each case states its expected tier so the fixture cannot silently drift onto
// another path: if a table edit moved sr-Latn/sr-Cyrl off TierSameLanguage, the
// benchmark would keep running and would quietly chart the other-script branch
// under the close-script name, and a chart series name is permanent.
type tierPair struct {
	name string
	want string
	have string
	tier langtag.Tier
	// charted records whether this pair gets its own benchmark series. Every
	// pair is measured for allocations, because each is a documented tier case a
	// consumer can reach; only pairs with a DISTINCT cost path are charted,
	// because a second series over the same branch buys a second chart to read
	// and another ~1.2 s of every weekly run.
	charted bool
}

// tierPairs covers every tier the ladder defines, plus the two extra branches
// inside TierSameLanguage. Built as a literal, deliberately: the weekly tracker
// runs with -run='^$', so no Test function executes first, and a benchmark
// leaning on state that a Test populated would pass locally and fail there.
//
// The three same-language entries reach that tier by different work: a
// macrolanguage fold, a region-only difference, and a CLDR-vouched script pair.
// The last is the only linear scan anywhere in the comparison path
// (scriptsReadAsOne in scripts.go) and measured four times the cost of the other
// two, so it is charted and the region case is not — region and macro fold are
// the same branch, "one language, same script".
//
// shared_literacy is likewise uncharted: reaching it is the same single map
// lookup that reaches intelligible, and the tier differs only by which value was
// stored in the table at construction. The tier is not untested — the allocation
// contract covers it and compare_test.go owns its correctness.
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

// TestPreferenceCompareIsAllocationFree pins the cost of the innermost loop of
// the library.
//
// Every exported method a comparison site calls is covered, not just Compare,
// because a consumer's loop body is realistically Compare or Match plus Reason on
// the surprising answers, and an allocation in any of them is multiplied by
// tracks times episodes. Prefer is included because the build-once idiom the API
// is shaped around depends on construction being cheap enough that rebuilding is
// a style question rather than a cost one.
//
// The zero-tag case is its own subtest: it is the answer for every input a
// consumer failed to parse, and compare.go returns TierNone before touching the
// table, so it must stay the cheapest comparison in the library rather than
// becoming the most expensive.
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

// TestBestDoesNotAllocateWhenNothingMatches pins the fail-closed path's cost.
//
// Best returns a slice, so it allocates as soon as it has something to return,
// and it is not expected to be free in general (measured: one allocation for a
// single match). The property that matters is the other direction. A consumer
// asks Best about every episode, and for a library of tracks tagged in languages
// nobody asked for the answer is "nothing close enough" — so the no-match path is
// the one that runs most, and it must cost nothing but the scan. compare.go leaves
// the output slice nil until a candidate lands within the floor; this keeps it
// that way.
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

// BenchmarkCompareFallbackBucket answers whether grading walks the fallback
// table, by charting one lookup against buckets 256 times apart in size.
//
// Today it does not walk it: compare reads the wanted language's bucket with a
// map index and the available language out of it with a second one, so these two
// series should sit on top of each other. That flatness is the point of the
// family. Replacing either map with a slice is an easy-looking change — the
// built-in table has five entries and a scan of five beats a map — and it would
// send this family super-linear while every other series in the package stayed
// put, which names the regression precisely.
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

// BenchmarkPreferenceAcrossTracks charts the shape a consumer actually runs: one
// wanted language judged against every track an episode offers.
//
// reuse builds the Preference once outside the track loop, which is the idiom the
// API is designed around and what both consumers do. It is parameterised by track
// count so per-track cost is separable from per-episode overhead, and so a
// comparison loop that became quadratic in candidates would show as a
// super-linear jump between the sizes.
//
// rebuild constructs a Preference per candidate, which is what a caller does when
// it threads the wanted Tag down instead of the Preference. It is charted at the
// larger size only: measured over three runs, rebuilding costs 1 to 2 ns per
// candidate — 5 to 6% of the loop, and zero allocations either way — so at 8
// tracks the difference sits inside this machine's own run-to-run spread and a
// series for it would chart noise forever. The gap is what the idiom costs,
// charted rather than argued so the answer stays current: if constructing a
// Preference ever stops being a struct copy, this pair separates first.
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

// benchTracks builds n parsed tags by cycling through langs, which is the shape
// of a real candidate list: a handful of languages, several of them repeated
// across tracks.
//
// It is a function rather than a package-level table so nothing here depends on
// initialization order, and it parses during setup so no benchmark pays parse
// cost inside its timed loop. Keeping the two cost centres apart is the whole
// point of splitting this file from tag_bench_test.go.
func benchTracks(n int, langs ...string) []langtag.Tag {
	out := make([]langtag.Tag, 0, n)
	for i := range n {
		out = append(out, langtag.MustParse(langs[i%len(langs)]))
	}
	return out
}

// benchFallbackTable builds a table of n valid entries that all name "no" as the
// wanted language, so every entry lands in one bucket and the entry a lookup has
// to find sits in a bucket of size n. That is the shape in which a scan would be
// visible; a table of n entries spread over n buckets would not show one.
//
// The stand-in languages are synthetic three-letter subtags. The table accepts
// them because entries are bare strings matched against [langtag.Tag.Language]
// and are never validated against the IANA registry, and none of them is the
// folded language of any real tag these benchmarks parse, so the only reachable
// entry is the last one: "no" -> "sv".
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
