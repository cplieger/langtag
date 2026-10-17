// Package multiscript owns the rule behind langtag's Tag.MultiScript, so the
// script table generator's refresh report names exactly the languages whose
// answer a table change moves.
package multiscript

import (
	"slices"

	"golang.org/x/text/language"
)

// Subtags returns t's primary language subtag and that subtag after
// macrolanguage folding (kmr and ku, cmn and zh): the two keys a tag's script
// lookup and its MultiScript answer read.
func Subtags(t language.Tag) (own, macro string) {
	base, _, _ := t.Raw()
	// Canonicalize returns a nil error on every path in x/text v0.43.0:
	// https://cs.opensource.google/go/x/text/+/refs/tags/v0.43.0:language/language.go
	folded, _ := language.Macro.Canonicalize(t)
	macroBase, _, _ := folded.Raw()
	return base.String(), macroBase.String()
}

// Holds reports whether a tag whose Subtags are own and macro is MultiScript
// under a script table for which hasRegionRows reports the languages keying
// at least one region row.
func Holds(hasRegionRows func(language string) bool, own, macro string) bool {
	return hasRegionRows(own) || hasRegionRows(macro)
}

// Languages returns, sorted and once each, the primary subtag of every
// language that Holds under hasRegionRows, a language reached only through
// its macrolanguage (kmr through ku) included. It reads every two- and
// three-letter candidate x/text parses, which is every language it knows:
// BCP 47 reserves four letters and registers no five- to eight-letter one.
func Languages(hasRegionRows func(language string) bool) []string {
	var out []string
	for _, c := range candidates() {
		t, err := language.Parse(c)
		if err != nil {
			continue
		}
		if own, macro := Subtags(t); Holds(hasRegionRows, own, macro) {
			out = append(out, own)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// candidates returns every lowercase two- and three-letter string.
func candidates() []string {
	out := make([]string, 0, 26*26+26*26*26)
	for a := byte('a'); a <= 'z'; a++ {
		for b := byte('a'); b <= 'z'; b++ {
			out = append(out, string([]byte{a, b}))
			for c := byte('a'); c <= 'z'; c++ {
				out = append(out, string([]byte{a, b, c}))
			}
		}
	}
	return out
}
