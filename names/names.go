// Package names gives a langtag Tag its English name, composed the way
// CLDR en's locale display pattern composes one: "Serbian (Latin)",
// "Chinese (Traditional, Hong Kong SAR China)". The table is generated from
// pinned CLDR and ISO 639-2 files and checked in, so importing the package
// does no work at init.
package names

//go:generate go run ../internal/gen/names -root ..

import (
	"slices"
	"strings"

	"github.com/cplieger/langtag/v3"
)

// entry is one table row: a subtag and its English name.
type entry struct{ code, name string }

// English composes t's name with CLDR en's locale display pattern:
// "Language", "Language (Script)", "Language (Region)" or
// "Language (Script, Region)", from the stated subtags only, so a bare "zh"
// is "Chinese". The language name is CLDR en's entry for the language subtag
// alone, never a combined one ("Brazilian Portuguese"), else the ISO 639-2
// English name, else the name of a code Parse canonicalizes to the subtag
// ("bh" from ISO's "bih"); ok is false when none names the subtag or t is
// zero. A script or region CLDR en does not name is written as its code.
func English(t langtag.Tag) (name string, ok bool) {
	if t.IsZero() {
		return "", false
	}
	code, _, _ := strings.Cut(t.String(), "-")
	language, ok := lookup(languages[:], code)
	if !ok {
		return "", false
	}
	detail := ""
	if s := t.StatedScript(); s != "" {
		detail = nameOrCode(scripts[:], s)
	}
	if r := t.Region(); r != "" {
		region := nameOrCode(regions[:], r)
		if detail == "" {
			detail = region
		} else {
			detail = compose(localeSeparator, detail, region)
		}
	}
	if detail == "" {
		return language, true
	}
	return compose(localePattern, language, detail), true
}

// Language is one language subtag the table names and its English name.
type Language struct {
	Code, Name string
}

// Languages returns every language subtag the table names with its English
// name, sorted by code, in a slice the caller owns. It lists the subtags as
// CLDR and ISO 639-2 spell them, so it holds codes Parse canonicalizes to
// another, each with its own name: {"tl", "Tagalog"} beside
// {"fil", "Filipino"}.
func Languages() []Language {
	out := make([]Language, len(languages))
	for i, e := range &languages {
		out[i] = Language{Code: e.code, Name: e.name}
	}
	return out
}

func lookup(table []entry, code string) (string, bool) {
	i, found := slices.BinarySearchFunc(table, code, func(e entry, c string) int { return strings.Compare(e.code, c) })
	if !found {
		return "", false
	}
	return table[i].name, true
}

func nameOrCode(table []entry, code string) string {
	if name, ok := lookup(table, code); ok {
		return name
	}
	return code
}

// compose fills a CLDR pattern's {0} and {1} in one pass, so a brace in a
// name is never read as a placeholder.
func compose(pattern, first, second string) string {
	return strings.NewReplacer("{0}", first, "{1}", second).Replace(pattern)
}
