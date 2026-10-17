package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v3"
)

// This file gates Parse's allocations and feeds BenchmarkParse because Parse
// delegates grammar and canonicalization to golang.org/x/text (tag.go): a
// dependency bump can move the cost with no langtag diff, and both consumers
// parse once per track per episode. The contract: accessors and a blank input
// never allocate, Parse's count is pinned per region shape, and Valid costs no
// more than Parse. None of these tests calls t.Parallel: testing.AllocsPerRun
// panics while a parallel test runs (Go 1.25), since the process-wide malloc
// count would include the sibling's: https://go.dev/doc/go1.25#testing.

// parseSink* consume every result these benchmarks and contracts produce.
// b.Loop keeps the call itself from being elided, but a return value nothing
// reads is still eliminable, and a benchmark measuring an elided call charts a
// constant forever.
var (
	parseSinkTag    langtag.Tag
	parseSinkString string
	parseSinkBool   bool
)

// TestTagAccessorsAreAllocationFree pins that Tag's accessors are readable
// without touching the heap, per tag.go's documented shape (comparable,
// copyable, no reference to the parsing library). The zero Tag is checked
// alongside a populated one because it is what Parse returns for every
// rejected input — the path a library full of untagged tracks runs most.
func TestTagAccessorsAreAllocationFree(t *testing.T) {
	tags := map[string]langtag.Tag{
		"populated":    langtag.MustParse("zh-Hant-HK"),
		"three-letter": langtag.MustParse("yue"),
		"zero":         {},
	}
	for name, tag := range tags {
		t.Run(name, func(t *testing.T) {
			accessors := map[string]func(){
				"String":       func() { parseSinkString = tag.String() },
				"Language":     func() { parseSinkString = tag.Language() },
				"Script":       func() { parseSinkString = tag.Script() },
				"StatedScript": func() { parseSinkString = tag.StatedScript() },
				"Region":       func() { parseSinkString = tag.Region() },
				"ISO3":         func() { parseSinkString = tag.ISO3() },
				"MultiScript":  func() { parseSinkBool = tag.MultiScript() },
				"IsZero":       func() { parseSinkBool = tag.IsZero() },
			}
			for method, call := range accessors {
				if got := testing.AllocsPerRun(100, call); got != 0 {
					t.Errorf("Tag.%s() on the %s tag %q allocated %v times per run, want 0",
						method, name, tag.String(), got)
				}
			}
		})
	}
}

// TestParseRejectsBlankInputWithoutAllocating gates the cost of the most
// common rejected input: a media file with no language attribute. Parse
// trims and returns before calling language.Parse (tag.go), which is a time
// optimization guarded by BenchmarkParse/empty; this test guards the
// separate property that nothing on the rejection path ever allocates.
func TestParseRejectsBlankInputWithoutAllocating(t *testing.T) {
	for _, raw := range []string{"", " ", "\t\n  "} {
		if got := testing.AllocsPerRun(100, func() {
			parseSinkTag, parseSinkBool = langtag.Parse(raw)
		}); got != 0 {
			t.Errorf("Parse(%q) allocated %v times per run, want 0: rejecting a blank "+
				"identifier must not allocate", raw, got)
		}
	}
}

// TestParseAllocations pins Parse's allocation count per region shape: a
// numeric region (es-419) and an alpha one (pt-BR) take different x/text
// paths, so a regression in one hides in the other's count. A lower count
// after a dependency bump is a win: lower the pin to it.
func TestParseAllocations(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want float64
		// raceSlack widens want to a ceiling under -race, CI's only test run:
		// x/text formats a numeric region with fmt.Sprintf, whose printer
		// comes from a sync.Pool, and the race runtime drops one Put in four
		// ($GOROOT/src/sync/pool.go, Pool.Put), so es-419 measures 8 or 9
		// while an alpha region never varies: only the numeric pin is loosened.
		raceSlack float64
	}{
		"numeric region": {raw: "es-419", want: 8, raceSlack: 1},
		"alpha region":   {raw: "pt-BR", want: 4},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := testing.AllocsPerRun(100, func() {
				parseSinkTag, parseSinkBool = langtag.Parse(tc.raw)
			})
			if raceEnabled {
				if ceiling := tc.want + tc.raceSlack; got > ceiling {
					t.Errorf("Parse(%q) allocated %v times per run under -race, want at most %v", tc.raw, got, ceiling)
				}
			} else if got != tc.want {
				t.Errorf("Parse(%q) allocated %v times per run, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestValidCostsNoMoreThanParse pins Valid as a wrapper rather than a second
// implementation. The assertion is one-sided: Valid costing MORE than Parse
// is waste, since the only thing a caller reads is a bool Parse already
// computed. Valid costing less would be a behavioral divergence, which
// belongs to tag_test.go, not an allocation count.
func TestValidCostsNoMoreThanParse(t *testing.T) {
	const raw = "ger"

	parse := testing.AllocsPerRun(100, func() {
		parseSinkTag, parseSinkBool = langtag.Parse(raw)
	})
	valid := testing.AllocsPerRun(100, func() {
		parseSinkBool = langtag.Valid(raw)
	})
	t.Logf("Parse(%q) allocates %v times per run, Valid(%q) %v", raw, parse, raw, valid)

	if valid > parse {
		t.Errorf("Valid(%q) allocated %v times per run, want no more than Parse(%q)'s %v: "+
			"Valid is Parse with the Tag discarded", raw, valid, raw, parse)
	}
}

// BenchmarkParse charts one series per input class, because the classes take
// different paths through golang.org/x/text and a regression confined to one
// would vanish in an average. plain_alpha2 and canonicalized_alpha3 both
// produce "de", so their gap is canonicalization's cost; empty is the
// guarded reject path, so a reordered short-circuit shows as that series
// tripling.
func BenchmarkParse(b *testing.B) {
	// Built as a literal, deliberately: the weekly tracker runs with -run='^$',
	// so no Test function executes first and a benchmark that leaned on
	// state a Test populated would pass locally and fail there.
	classes := []struct {
		name string
		raw  string
	}{
		{"plain_alpha2", "de"},
		{"canonicalized_alpha3", "ger"},
		{"script_and_region", "zh-Hant-HK"},
		{"grandfathered", "i-klingon"},
		{"already_canonical_full", "pt-BR"},
		{"numeric_region", "es-419"},
		{"unparseable", "notalanguage"},
		{"empty", ""},
	}
	for _, class := range classes {
		b.Run(class.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				parseSinkTag, parseSinkBool = langtag.Parse(class.raw)
			}
		})
	}
}
