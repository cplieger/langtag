package langtag

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3/internal/multiscript"
)

// regionLanguages returns, sorted, the languages keying a region row.
func regionLanguages() []string {
	out := make([]string, 0, len(regionScripts))
	for _, r := range regionScripts {
		out = append(out, r.language)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestScriptTableSorted pins the order resolveScript's binary searches need:
// an out-of-order or repeated row makes a lookup miss a row it holds.
func TestScriptTableSorted(t *testing.T) {
	t.Parallel()
	for i := 1; i < len(languageScripts); i++ {
		if languageScripts[i-1].language >= languageScripts[i].language {
			t.Errorf("languageScripts[%d] = %q follows %q", i, languageScripts[i].language, languageScripts[i-1].language)
		}
	}
	for i := 1; i < len(regionScripts); i++ {
		if compareRegionRows(regionScripts[i-1], regionScripts[i]) >= 0 {
			t.Errorf("regionScripts[%d] = %s_%s follows %s_%s", i,
				regionScripts[i].language, regionScripts[i].region, regionScripts[i-1].language, regionScripts[i-1].region)
		}
	}
}

// TestMultiScript pins that the set is keyed by the tag's subtag or its
// Language(): kmr, zsm and uzn have no rows of their own and vary through ku,
// ms and uz.
func TestMultiScript(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"zh": true, "sr": true, "ku": true, "yue": true,
		"kmr": true, "zsm": true, "uzn": true,
		"zh-Hant": true, "sr-Latn-RS": true,
		"pt": false, "cnr": false, "en": false, "pt-BR": false,
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got := MustParse(raw).MultiScript(); got != want {
				t.Errorf("MustParse(%q).MultiScript() = %v, want %v", raw, got, want)
			}
		})
	}
	t.Run("zero tag", func(t *testing.T) {
		t.Parallel()
		if (Tag{}).MultiScript() {
			t.Error("Tag{}.MultiScript() = true, want false")
		}
	})
	// The refresh report's set (multiscript.Languages) is exactly the
	// languages Parse answers MultiScript true for, at whatever CLDR release
	// the table holds.
	t.Run("the refresh report's set", func(t *testing.T) {
		t.Parallel()
		want := map[string]bool{}
		for _, raw := range lowerCandidates() {
			if tag, ok := Parse(raw); ok && tag.MultiScript() {
				own, _, _ := strings.Cut(tag.String(), "-")
				want[own] = true
			}
		}
		got := multiscript.Languages(hasRegionRows)
		if !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
			t.Errorf("multiscript.Languages = %v, want the subtags Parse answers MultiScript for: %v", got, slices.Sorted(maps.Keys(want)))
		}
		for _, lang := range append(regionLanguages(), "kmr", "zsm", "uzn") {
			if !slices.Contains(got, lang) {
				t.Errorf("multiscript.Languages lacks %s", lang)
			}
		}
	})
}

// regionLanguagesCopy are the 24 languages CLDR release-48's
// likelySubtags.xml gives a language-region row, and multiScriptCopy the
// 30 Tag.MultiScript answers true for: those and the six that reach their
// rows through Language(). Both are oracles, never read from the table.
var (
	regionLanguagesCopy = []string{
		"abq", "az", "ha", "hak", "kk", "ku", "ky", "lzz", "mey", "mn", "ms", "nan",
		"pa", "pi", "pnt", "sd", "sr", "stu", "tg", "ug", "unr", "uz", "yue", "zh",
	}
	multiScriptCopy = []string{
		"abq", "az", "azj", "cmn", "ha", "hak", "khk", "kk", "kmr", "ku", "ky", "lzz", "mey", "mn", "ms",
		"nan", "pa", "pi", "pnt", "sd", "sr", "stu", "tg", "ug", "unr", "uz", "uzn", "yue", "zh", "zsm",
	}
)

// TestMultiScriptMatchesCopy pins the set at the pinned release: a refresh that
// moves a language in or out of it fails here until the copy is updated.
func TestMultiScriptMatchesCopy(t *testing.T) {
	t.Parallel()
	if got := regionLanguages(); !slices.Equal(got, regionLanguagesCopy) {
		t.Errorf("languages with a region row = %d %v, want the copy's %d %v", len(got), got, len(regionLanguagesCopy), regionLanguagesCopy)
	}
	if got := multiscript.Languages(hasRegionRows); !slices.Equal(got, multiScriptCopy) {
		t.Errorf("multiscript.Languages = %d %v, want the copy's %d %v", len(got), got, len(multiScriptCopy), multiScriptCopy)
	}
}

// lowerCandidates returns every lowercase two- and three-letter string.
func lowerCandidates() []string {
	var out []string
	for a := 'a'; a <= 'z'; a++ {
		for b := 'a'; b <= 'z'; b++ {
			out = append(out, string([]rune{a, b}))
			for c := 'a'; c <= 'z'; c++ {
				out = append(out, string([]rune{a, b, c}))
			}
		}
	}
	return out
}
