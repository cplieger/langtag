package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v2"
)

func TestCompare(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want, have string
		tier       langtag.Tier
	}{
		// Tier 0: one language, one canonical tag, several published spellings.
		"same tag":             {"en", "en", langtag.TierIdentical},
		"german code systems":  {"ger", "deu", langtag.TierIdentical},
		"chinese code systems": {"chi", "zho", langtag.TierIdentical},
		"norwegian umbrella":   {"nor", "no", langtag.TierIdentical},
		"deprecated hebrew":    {"iw", "he", langtag.TierIdentical},
		"tagalog and filipino": {"tl", "fil", langtag.TierIdentical},

		// Tier 1, macrolanguage folding. This row is the reported bug.
		"bokmal wants norwegian": {"nob", "nor", langtag.TierSameLanguage},
		"norwegian wants bokmal": {"nor", "nob", langtag.TierSameLanguage},
		"mandarin wants chinese": {"cmn", "zh", langtag.TierSameLanguage},
		"chinese wants mandarin": {"zh", "cmn", langtag.TierSameLanguage},
		"msa wants arabic":       {"arb", "ar", langtag.TierSameLanguage},
		"swahili":                {"swh", "sw", langtag.TierSameLanguage},

		// Tier 1, region only. Reading is unaffected by a region subtag.
		"european vs latin american spanish": {"es-ES", "es-419", langtag.TierSameLanguage},
		"brazilian vs european portuguese":   {"pt-BR", "pt-PT", langtag.TierSameLanguage},
		"british vs american english":        {"en-GB", "en-US", langtag.TierSameLanguage},
		"bare vs regional danish":            {"da", "da-DK", langtag.TierSameLanguage},
		"bare chinese vs simplified":         {"zh", "zh-Hans", langtag.TierSameLanguage},
		"bare serbian vs cyrillic":           {"sr", "sr-Cyrl", langtag.TierSameLanguage},
		"serbian scripts read as one":        {"sr-Cyrl", "sr-Latn", langtag.TierSameLanguage},
		"serbian scripts, other direction":   {"sr-Latn", "sr-Cyrl", langtag.TierSameLanguage},

		// Tier 2: one language, different script.
		"simplified vs traditional chinese":          {"zh-Hans", "zh-Hant", langtag.TierOtherScript},
		"traditional vs simplified chinese":          {"zh-Hant", "zh-Hans", langtag.TierOtherScript},
		"uzbek scripts":                              {"uz-Latn", "uz-Cyrl", langtag.TierOtherScript},
		"mandarin traditional vs chinese simplified": {"cmn-Hant", "zh-Hans", langtag.TierOtherScript},

		// Tier 3: a different language, from the curated table.
		"bokmal wants nynorsk":    {"nb", "nn", langtag.TierIntelligible},
		"nynorsk wants bokmal":    {"nn", "nb", langtag.TierIntelligible},
		"norwegian wants nynorsk": {"nor", "nno", langtag.TierIntelligible},
		"norwegian wants danish":  {"no", "da", langtag.TierIntelligible},
		"danish wants norwegian":  {"da", "nb", langtag.TierIntelligible},
		"croatian wants bosnian":  {"hr", "bs", langtag.TierIntelligible},
		"bosnian wants croatian":  {"bs", "hr", langtag.TierIntelligible},
		"czech wants slovak":      {"cs", "sk", langtag.TierIntelligible},
		"slovak wants czech":      {"sk", "cs", langtag.TierIntelligible},

		// Tier 4: a different language reachable only because readers of the
		// first are broadly literate in the second. One direction only.
		"catalan wants spanish": {"ca", "es", langtag.TierSharedLiteracy},

		// Tier 5: unrelated, or related in ways this package refuses to act on.
		"norwegian wants swedish":        {"no", "sv", langtag.TierNone},
		"danish wants swedish":           {"da", "sv", langtag.TierNone},
		"spanish wants catalan":          {"es", "ca", langtag.TierNone},
		"serbian wants croatian":         {"sr", "hr", langtag.TierNone},
		"hindi wants urdu":               {"hi", "ur", langtag.TierNone},
		"polish wants czech":             {"pl", "cs", langtag.TierNone},
		"galician wants spanish":         {"gl", "es", langtag.TierNone},
		"cantonese wants mandarin":       {"yue", "cmn", langtag.TierNone},
		"egyptian wants standard arabic": {"arz", "arb", langtag.TierNone},
		"english wants japanese":         {"en", "ja", langtag.TierNone},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := langtag.MustParse(tc.want)
			have := langtag.MustParse(tc.have)
			if got := langtag.Prefer(want).Compare(have); got != tc.tier {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v", tc.want, tc.have, got, tc.tier)
			}
		})
	}
}

// TestCompareExcludesSharedLiteracy is the load-bearing regression guard.
//
// CLDR's distance data rates every pair below as close, because the population
// that speaks the first language is broadly literate in the second. That is
// accurate sociolinguistics and an unacceptable substitution rule: a viewer who
// chose Tamil subtitles did not ask for English ones. This package excludes the
// whole family by construction, because tiers 0 through 2 cannot express a
// cross-language relationship at all.
//
// If this test fails, someone has wired a CLDR matcher back in, or added a
// tier-3 entry from that family. Both need a deliberate decision, not a patch.
func TestCompareExcludesSharedLiteracy(t *testing.T) {
	t.Parallel()
	pairs := map[string][2]string{
		"tamil and english":        {"ta", "en"},
		"bengali and english":      {"bn", "en"},
		"telugu and english":       {"te", "en"},
		"urdu and english":         {"ur", "en"},
		"sinhala and english":      {"si", "en"},
		"burmese and english":      {"my", "en"},
		"khmer and english":        {"km", "en"},
		"nepali and english":       {"ne", "en"},
		"georgian and english":     {"ka", "en"},
		"welsh and english":        {"cy", "en"},
		"irish and english":        {"ga", "en"},
		"maltese and english":      {"mt", "en"},
		"basque and spanish":       {"eu", "es"},
		"galician and spanish":     {"gl", "es"},
		"belarusian and russian":   {"be", "ru"},
		"tatar and russian":        {"tt", "ru"},
		"kazakh and russian":       {"kk", "ru"},
		"armenian and russian":     {"hy", "ru"},
		"kurdish and turkish":      {"ku", "tr"},
		"luxembourgish and german": {"lb", "de"},
		"romansh and german":       {"rm", "de"},
		"breton and french":        {"br", "fr"},
		"corsican and french":      {"co", "fr"},
		"javanese and indonesian":  {"jv", "id"},
		"sundanese and indonesian": {"su", "id"},
		"gujarati and hindi":       {"gu", "hi"},
		"marathi and hindi":        {"mr", "hi"},
		"frisian and dutch":        {"fy", "nl"},
		"afrikaans and dutch":      {"af", "nl"},
		"cebuano and tagalog":      {"ceb", "tl"},
		"macedonian and bulgarian": {"mk", "bg"},
		"estonian and finnish":     {"et", "fi"},
		"malay and indonesian":     {"ms", "id"},
	}
	for name, p := range pairs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, b := langtag.MustParse(p[0]), langtag.MustParse(p[1])
			if got := langtag.Prefer(a).Compare(b); got != langtag.TierNone {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v (shared literacy is not interchangeability)",
					p[0], p[1], got, langtag.TierNone)
			}
			if got := langtag.Prefer(b).Compare(a); got != langtag.TierNone {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v (shared literacy is not interchangeability)",
					p[1], p[0], got, langtag.TierNone)
			}
		})
	}
}

// TestCompareIsDirected pins that the direction carried by the Preference role
// is load-bearing, so the asymmetry is a contract rather than an accident of
// the current table.
func TestCompareIsDirected(t *testing.T) {
	t.Parallel()
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	if got := langtag.Prefer(ca).Compare(es); got != langtag.TierSharedLiteracy {
		t.Errorf("Prefer(ca).Compare(es) = %v, want %v (a Catalan reader reads Spanish)", got, langtag.TierSharedLiteracy)
	}
	if got := langtag.Prefer(es).Compare(ca); got != langtag.TierNone {
		t.Errorf("Prefer(es).Compare(ca) = %v, want %v (a Spanish reader does not read Catalan)", got, langtag.TierNone)
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()
	nob, nor := langtag.MustParse("nob"), langtag.MustParse("nor")
	nn := langtag.MustParse("nn")
	hant, hans := langtag.MustParse("zh-Hant"), langtag.MustParse("zh-Hans")

	cases := map[string]struct {
		want, have langtag.Tag
		floor      langtag.Tier
		match      bool
	}{
		"identical floor rejects macrolanguage": {nob, nor, langtag.TierIdentical, false},
		"same-language floor accepts it":        {nob, nor, langtag.TierSameLanguage, true},
		"same-language floor rejects script":    {hans, hant, langtag.TierSameLanguage, false},
		"other-script floor accepts script":     {hans, hant, langtag.TierOtherScript, true},
		"other-script floor rejects nynorsk":    {nob, nn, langtag.TierOtherScript, false},
		"intelligible floor accepts nynorsk":    {nob, nn, langtag.TierIntelligible, true},
		"intelligible floor rejects catalan":    {langtag.MustParse("ca"), langtag.MustParse("es"), langtag.TierIntelligible, false},
		"shared-literacy floor accepts catalan": {langtag.MustParse("ca"), langtag.MustParse("es"), langtag.TierSharedLiteracy, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := langtag.Prefer(tc.want).Match(tc.have, tc.floor); got != tc.match {
				t.Errorf("Prefer(%q).Match(%q, %v) = %v, want %v",
					tc.want, tc.have, tc.floor, got, tc.match)
			}
		})
	}
}

// TestMatchIsMonotonicInFloor pins the ordering property the tier scale
// promises: a pair accepted at one floor is accepted at every looser floor.
// Without it the tiers would be labels rather than a scale.
func TestMatchIsMonotonicInFloor(t *testing.T) {
	t.Parallel()
	tags := []string{
		"en", "en-GB", "nob", "nor", "nn", "da", "sv", "zh", "zh-Hans",
		"zh-Hant", "cmn", "yue", "ca", "es", "es-419", "hr", "bs", "sr", "ta",
	}
	floors := []langtag.Tier{
		langtag.TierIdentical, langtag.TierSameLanguage,
		langtag.TierOtherScript, langtag.TierIntelligible, langtag.TierSharedLiteracy,
	}
	for _, a := range tags {
		for _, b := range tags {
			p := langtag.Prefer(langtag.MustParse(a))
			have := langtag.MustParse(b)
			matchedAt := -1
			for i, floor := range floors {
				if p.Match(have, floor) {
					matchedAt = i
					break
				}
			}
			if matchedAt < 0 {
				continue
			}
			for _, floor := range floors[matchedAt:] {
				if !p.Match(have, floor) {
					t.Errorf("Prefer(%q).Match(%q, %v) = false, want true (accepted at %v, so every looser floor must accept)",
						a, b, floor, floors[matchedAt])
				}
			}
		}
	}
}
