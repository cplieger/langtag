package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v3"
)

// coverCase is one directed predicate check: want is the target, have the
// offered tag.
type coverCase struct {
	want, have string
	covers     bool
}

// TestCoversScript pins the script half of a match: a stated script is
// required exactly, an unstated one admits the resolved script and its close
// pair, and an unknown script matches nothing but the identical tag.
func TestCoversScript(t *testing.T) {
	t.Parallel()
	cases := map[string]coverCase{
		"bare serbian, croatian latin":     {"sr", "hr", true},
		"bare serbian, cyrillic":           {"sr", "sr-Cyrl", true},
		"bare serbian, latin":              {"sr", "sr-Latn", true},
		"bare uzbek refuses cyrillic":      {"uz", "uz-Cyrl", false},
		"serbian latin refuses cyrillic":   {"sr-Latn", "sr-Cyrl", false},
		"serbian latin, croatian":          {"sr-Latn", "hr", true},
		"bare chinese refuses traditional": {"zh", "zh-Hant", false},
		"traditional, resolved taiwan":     {"zh-Hant", "zh-TW", true},
		"stated script ignores region":     {"zh-Hant-HK", "zh-Hant-TW", true},
		"resolved script ignores region":   {"zh-HK", "zh-TW", true},
		"montenegrin covers itself":        {"cnr", "cnr", true},
		"montenegrin refuses latin":        {"cnr", "cnr-Latn", false},
		"montenegrin refuses a region":     {"cnr", "cnr-ME", false},
		"cyrillic refuses unknown":         {"cnr-Cyrl", "cnr", false},
		"stated unknown refuses unknown":   {"zh-Zzzz", "cnr", false},
		"stated unknown covers itself":     {"zh-Zzzz-TW", "zh-Zzzz-TW", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, have := langtag.MustParse(tc.want), langtag.MustParse(tc.have)
			if got := want.CoversScript(have); got != tc.covers {
				t.Errorf("Parse(%q).CoversScript(%q) = %v, want %v", tc.want, tc.have, got, tc.covers)
			}
		})
	}
}

// TestCoversRegion pins the region half: a target stating no region covers
// any region, and one stating a region is covered by that region or one it
// contains. An unplaced region (ZZ, CLDR's unknown region) counts as none on
// either side, as it does for TierOtherRegion.
func TestCoversRegion(t *testing.T) {
	t.Parallel()
	cases := map[string]coverCase{
		"latin america covers mexico":      {"es-419", "es-MX", true},
		"mexico refuses latin america":     {"es-MX", "es-419", false},
		"brazil refuses bare portuguese":   {"pt-BR", "pt", false},
		"bare portuguese covers brazil":    {"pt", "pt-BR", true},
		"bare portuguese covers bare":      {"pt", "pt", true},
		"no region ignores language":       {"pt", "en-US", true},
		"containment ignores language":     {"es-419", "pt-BR", true},
		"no region ignores script":         {"pt", "zh-Hant-TW", true},
		"containment ignores script":       {"es-419", "zh-Hant-BR", true},
		"same region":                      {"es-419", "es-419", true},
		"spain refuses mexico":             {"es-ES", "es-MX", false},
		"latin america refuses spain":      {"es-419", "es-ES", false},
		"grouping row covers its members":  {"es-UN", "es-MX", true},
		"unknown region states none":       {"es-ZZ", "es-MX", true},
		"private-use region states none":   {"pt-QM", "pt-BR", true},
		"mexico refuses an unknown region": {"es-MX", "es-ZZ", false},
		"placed user-assigned code counts": {"en-XK", "en-US", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, have := langtag.MustParse(tc.want), langtag.MustParse(tc.have)
			if got := want.CoversRegion(have); got != tc.covers {
				t.Errorf("Parse(%q).CoversRegion(%q) = %v, want %v", tc.want, tc.have, got, tc.covers)
			}
		})
	}
}

// TestCoversRejectsTheZeroTag pins that both predicates treat the zero Tag
// as the tiers do: it names no language, so it covers and is covered by
// nothing, another zero Tag included.
func TestCoversRejectsTheZeroTag(t *testing.T) {
	t.Parallel()
	pt := langtag.MustParse("pt")
	pairs := map[string][2]langtag.Tag{
		"zero target": {{}, pt},
		"zero offer":  {pt, {}},
		"both zero":   {{}, {}},
	}
	for name, p := range pairs {
		if p[0].CoversScript(p[1]) {
			t.Errorf("%s: CoversScript = true, want false", name)
		}
		if p[0].CoversRegion(p[1]) {
			t.Errorf("%s: CoversRegion = true, want false", name)
		}
	}
}
