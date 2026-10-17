package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v3"
)

// TestCoversScriptNeverCrossesScripts is the property consumers rely on for
// "never crosses scripts": over every ordered pair of the parse corpus, two
// known scripts that differ and are not a close pair for the target's
// language never cover, and an unknown script on either side covers only the
// identical tag.
func TestCoversScriptNeverCrossesScripts(t *testing.T) {
	t.Parallel()
	var tags []langtag.Tag
	for _, raw := range append(append([]string{}, parseCorpus...), parseSeeds...) {
		if tag, ok := langtag.Parse(raw); ok {
			tags = append(tags, tag)
		}
	}
	closePair := func(lang, a, b string) bool {
		for _, cs := range langtag.CloseScripts() {
			if cs.Language == lang && ((cs.A == a && cs.B == b) || (cs.A == b && cs.B == a)) {
				return true
			}
		}
		return false
	}
	const unknown = "Zzzz"
	var unknownPairs, crossPairs int
	for _, want := range tags {
		for _, have := range tags {
			ws, hs := want.Script(), have.Script()
			got := want.CoversScript(have)
			switch {
			case ws == unknown || hs == unknown:
				unknownPairs++
				if got != (want == have) {
					t.Errorf("%q.CoversScript(%q) = %v with an unknown script, want %v (only the identical tag)",
						want, have, got, want == have)
				}
			case ws != hs && !closePair(want.Language(), ws, hs):
				crossPairs++
				if got {
					t.Errorf("%q.CoversScript(%q) = true across %s and %s, want false", want, have, ws, hs)
				}
			}
		}
	}
	// The corpus must reach both branches, or the property holds vacuously.
	if unknownPairs == 0 || crossPairs == 0 {
		t.Fatalf("corpus reached %d unknown-script and %d cross-script pairs, want both above 0", unknownPairs, crossPairs)
	}
}
