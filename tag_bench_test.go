package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v2"
)

// This file exists because the parse half of langtag is the only part of the
// library whose cost is not first-party. [langtag.Parse] delegates grammar,
// canonicalization and default-script resolution to golang.org/x/text (see
// tag.go), so a dependency bump can move this number without a line of langtag
// changing. That is precisely the regression a weekly benchmark tracker is good
// at catching and a pull-request diff cannot show.
//
// The cost matters because of how the library is called. Both consumers
// (plex-language-sync, subflux) parse once per available track per episode, so
// the per-call number multiplies by a media library's worth of tracks.
//
// Two kinds of check here, doing different jobs:
//
//   - The three Test functions GATE properties that were MEASURED to hold before
//     they were asserted. testing.AllocsPerRun is exact, so the assertions are
//     "== 0" rather than thresholds: a refactor that boxes a Tag into an
//     interface, builds a string in an accessor, or adds a diagnostic to the
//     reject path goes red at merge time instead of being noticed later in a
//     chart.
//   - BenchmarkParse feeds the tracker one series per INPUT CLASS rather than
//     one number for Parse. The classes are the paths that diverge inside
//     x/text, and a regression confined to one of them (say, a slower
//     grandfathered-tag lookup table) would be invisible in an average over all
//     of them.
//
// Two measurements taken while writing this, recorded so a later reader can tell
// a real move from a re-baselining. On go1.27.0 with x/text v0.41.0, Parse
// allocates 3 to 5 times for an accepted tag, once for an unparseable one and not
// at all for a blank one. And there is NO already-canonical fast path: "de" and
// "ger" both run the full
// language.Parse -> Compose -> Macro.Canonicalize -> Script pipeline and differ
// only by what canonicalization itself costs. Nothing short-circuits on a tag
// that is already in canonical form, so a future claim of such a path would be
// new behaviour rather than a restored property.
//
// None of the contracts below calls t.Parallel, deliberately:
// testing.AllocsPerRun measures a delta on a process-global allocation counter
// and pins GOMAXPROCS to 1 while it runs, so a concurrent sibling's allocations
// would be charged to whichever contract happened to be measuring.

// parseSink* consume every result these benchmarks and contracts produce.
// b.Loop keeps the call itself from being elided, but a return value nothing
// reads is still eliminable, and a benchmark measuring an elided call charts a
// constant forever.
var (
	parseSinkTag    langtag.Tag
	parseSinkString string
	parseSinkBool   bool
)

// TestTagAccessorsAreAllocationFree pins the cost half of Tag's documented shape.
// tag.go states that Tag "holds only strings, so it is comparable, copyable and
// carries no reference to the parsing library in its public shape", and a value
// type of that description must be readable without touching the heap.
//
// The zero Tag is checked alongside a populated one because it is the value
// [langtag.Parse] hands back for every rejected input, so it is the one a
// consumer reads on the failure path — the path that runs most often when a
// media library is full of untagged tracks.
func TestTagAccessorsAreAllocationFree(t *testing.T) {
	tags := map[string]langtag.Tag{
		"populated": langtag.MustParse("zh-Hant-HK"),
		"zero":      {},
	}
	for name, tag := range tags {
		t.Run(name, func(t *testing.T) {
			accessors := map[string]func(){
				"String":   func() { parseSinkString = tag.String() },
				"Language": func() { parseSinkString = tag.Language() },
				"Script":   func() { parseSinkString = tag.Script() },
				"IsZero":   func() { parseSinkBool = tag.IsZero() },
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

// TestParseRejectsBlankInputWithoutAllocating gates the cost of the most common
// rejected input there is. A media file with no language attribute reaches Parse
// as the empty string, so a consumer scanning a library runs this path far more
// often than it runs any real tag, and it must cost nothing.
//
// What this does and does not gate is worth stating, because the obvious reading
// is wrong. Parse trims and returns before calling language.Parse when nothing is
// left (tag.go), but deleting that early return does NOT redden this test:
// measured with the short-circuit removed, golang.org/x/text also rejects the
// empty string without allocating (0 allocations, 28.5 ns against 8.7 ns). So the
// early return is a time optimization and BenchmarkParse/empty is what guards it —
// losing it triples that series, well past the tracker's alert threshold. This
// test guards the other half: that nothing on the rejection path ever starts
// allocating, which is what a diagnostic message, a log line or a defensive copy
// of the caller's string would do.
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

// TestValidCostsNoMoreThanParse pins Valid as a wrapper rather than a second
// implementation. Valid is documented as configuration validation, so its
// absolute cost does not matter; what matters is that it stays Parse with the Tag
// discarded and adds nothing of its own.
//
// The assertion is deliberately one-sided, and the direction it catches is Valid
// costing MORE: work added here (a wrapped error, a formatted message, a
// defensive copy of the input) is pure waste, because the only thing the caller
// reads is a bool that Parse already computed. The other direction — a Valid made
// cheaper by skipping some of what Parse does — would be a behavioural
// divergence, two functions disagreeing about which identifiers are acceptable,
// and that belongs to the correctness tests in tag_test.go rather than to an
// allocation count. The absolute counts are a property of golang.org/x/text and
// are logged rather than asserted, so an upstream bump cannot redden this on its
// own.
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

// BenchmarkParse measures each input class that takes a different path through
// golang.org/x/text. Consumers call this once per available track per episode,
// so the number here is multiplied by every track in a media library.
//
// The classes, and the regression each one would show:
//
//   - plain_alpha2 and canonicalized_alpha3 both produce the tag "de". They are
//     charted as a PAIR on purpose: the gap between them is what
//     canonicalization costs, and it is the measurement that answers whether an
//     already-canonical fast path exists. It does not, today.
//   - script_and_region exercises the widest tag shape, where Compose has three
//     subtags to rebuild rather than one.
//   - grandfathered exercises the registry's irregular-tag lookup, a separate
//     table inside x/text that a bump can change independently of the rest.
//   - already_canonical_full is a multi-subtag tag that is already in canonical
//     form, so nothing is rewritten and only the pipeline's fixed cost remains.
//   - unparseable is the reject path a hostile or sloppy source drives. It must
//     stay cheap, because a scan over a library of badly tagged files runs it
//     more often than any accept path.
//   - empty is the blank short-circuit, and this series is what GUARDS it: the
//     early return is a time optimization no allocation count can see, so if it
//     is ever reordered away this series triples while nothing goes red.
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
