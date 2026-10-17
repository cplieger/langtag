package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v3"
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

		// Tier 1, region only, never across a CLDR cluster.
		"mexican vs latin american spanish": {"es-MX", "es-419", langtag.TierSameLanguage},
		"american vs canadian english":      {"en-US", "en-CA", langtag.TierSameLanguage},
		"bare vs regional danish":           {"da", "da-DK", langtag.TierSameLanguage},
		"bare chinese vs simplified":        {"zh", "zh-Hans", langtag.TierSameLanguage},
		"bare serbian vs cyrillic":          {"sr", "sr-Cyrl", langtag.TierSameLanguage},
		"serbian scripts read as one":       {"sr-Cyrl", "sr-Latn", langtag.TierSameLanguage},
		"serbian scripts, other direction":  {"sr-Latn", "sr-Cyrl", langtag.TierSameLanguage},

		// Tier 2: one language, two regional standards (TestOtherRegion).
		"european vs latin american spanish": {"es-ES", "es-419", langtag.TierOtherRegion},
		"brazilian vs european portuguese":   {"pt-BR", "pt-PT", langtag.TierOtherRegion},
		"british vs american english":        {"en-GB", "en-US", langtag.TierOtherRegion},

		// Tier 3: one language, different script.
		"simplified vs traditional chinese":          {"zh-Hans", "zh-Hant", langtag.TierOtherScript},
		"traditional vs simplified chinese":          {"zh-Hant", "zh-Hans", langtag.TierOtherScript},
		"uzbek scripts":                              {"uz-Latn", "uz-Cyrl", langtag.TierOtherScript},
		"mandarin traditional vs chinese simplified": {"cmn-Hant", "zh-Hans", langtag.TierOtherScript},

		// Tier 4: a different language, from the curated table.
		"bokmal wants nynorsk":    {"nb", "nn", langtag.TierIntelligible},
		"nynorsk wants bokmal":    {"nn", "nb", langtag.TierIntelligible},
		"norwegian wants nynorsk": {"nor", "nno", langtag.TierIntelligible},
		"norwegian wants danish":  {"no", "da", langtag.TierIntelligible},
		"danish wants norwegian":  {"da", "nb", langtag.TierIntelligible},
		"croatian wants bosnian":  {"hr", "bs", langtag.TierIntelligible},
		"bosnian wants croatian":  {"bs", "hr", langtag.TierIntelligible},
		"czech wants slovak":      {"cs", "sk", langtag.TierIntelligible},
		"slovak wants czech":      {"sk", "cs", langtag.TierIntelligible},
		"serbian wants croatian":  {"sr", "hr", langtag.TierIntelligible},
		"malay wants indonesian":  {"ms", "id", langtag.TierIntelligible},

		// Tier 5: a different language reachable only because readers of the
		// first are broadly literate in the second. One direction only.
		"catalan wants spanish": {"ca", "es", langtag.TierSharedLiteracy},

		// Tier 6: unrelated, or related in ways this package refuses to act on.
		"norwegian wants swedish":        {"no", "sv", langtag.TierNone},
		"danish wants swedish":           {"da", "sv", langtag.TierNone},
		"spanish wants catalan":          {"es", "ca", langtag.TierNone},
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

// TestOtherRegion pins the region tier to CLDR's paradigm clusters: two
// stated regions of one language grade other-region only across a cluster's
// edge, never inside one, never when one region contains the other, and never
// in place of a script difference.
func TestOtherRegion(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want, have string
		tier       langtag.Tier
	}{
		"latin american vs european spanish":    {"es-419", "es-ES", langtag.TierOtherRegion},
		"brazilian vs european portuguese":      {"pt-BR", "pt-PT", langtag.TierOtherRegion},
		"american vs british english":           {"en-US", "en-GB", langtag.TierOtherRegion},
		"moroccan vs egyptian arabic":           {"ar-MA", "ar-EG", langtag.TierOtherRegion},
		"hong kong vs taiwan traditional":       {"zh-Hant-HK", "zh-Hant-TW", langtag.TierOtherRegion},
		"stated script on one side only":        {"pt-BR", "pt-Latn-PT", langtag.TierOtherRegion},
		"resolved traditional script":           {"zh-HK", "zh-TW", langtag.TierOtherRegion},
		"macrolanguage member across clusters":  {"arb-MA", "ar-EG", langtag.TierOtherRegion},
		"latin america contains mexico":         {"es-419", "es-MX", langtag.TierSameLanguage},
		"both inside the us cluster":            {"en-US", "en-CA", langtag.TierSameLanguage},
		"both outside the americas":             {"pt-PT", "pt-AO", langtag.TierSameLanguage},
		"french has no cluster":                 {"fr-CA", "fr-FR", langtag.TierSameLanguage},
		"simplified has no cluster":             {"zh-Hans-HK", "zh-Hans-TW", langtag.TierSameLanguage},
		"world contains every region":           {"en-001", "en-US", langtag.TierSameLanguage},
		"northern america contains the us":      {"en-021", "en-US", langtag.TierSameLanguage},
		"northern africa contains morocco":      {"ar-015", "ar-MA", langtag.TierSameLanguage},
		"world contains mexico":                 {"es-001", "es-MX", langtag.TierSameLanguage},
		"caribbean straddles, counts outside":   {"en-029", "en-GB", langtag.TierSameLanguage},
		"caribbean straddles, against the us":   {"en-029", "en-US", langtag.TierOtherRegion},
		"northern africa straddles the maghreb": {"ar-015", "ar-SA", langtag.TierSameLanguage},
		"outlying oceania lost the us islands":  {"en-QO", "en-UM", langtag.TierOtherRegion},
		"outlying oceania lost bouvet":          {"es-QO", "es-BV", langtag.TierOtherRegion},
		"outlying oceania lost south georgia":   {"pt-QO", "pt-GS", langtag.TierOtherRegion},
		"oceania lost bouvet":                   {"es-009", "es-BV", langtag.TierOtherRegion},
		"micronesia contains the us islands":    {"en-057", "en-UM", langtag.TierSameLanguage},
		"united nations contains the us":        {"en-UN", "en-US", langtag.TierSameLanguage},
		"one region unstated":                   {"pt", "pt-BR", langtag.TierSameLanguage},
		"unknown region is unstated":            {"es-ZZ", "es-MX", langtag.TierSameLanguage},
		"private-use region is unstated":        {"es-XA", "es-MX", langtag.TierSameLanguage},
		"private-use aa is unstated":            {"en-AA", "en-US", langtag.TierSameLanguage},
		"private-use qm is unstated":            {"pt-QM", "pt-BR", langtag.TierSameLanguage},
		"kosovo is a placed region":             {"en-XK", "en-US", langtag.TierOtherRegion},
		"script difference wins":                {"zh-Hant-HK", "zh-Hans-TW", langtag.TierOtherScript},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, have := langtag.MustParse(tc.want), langtag.MustParse(tc.have)
			if got := langtag.Prefer(want).Compare(have); got != tc.tier {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v", tc.want, tc.have, got, tc.tier)
			}
			if got := langtag.Prefer(have).Compare(want); got != tc.tier {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v", tc.have, tc.want, got, tc.tier)
			}
		})
	}
}

// TestNewIntelligiblePairs pins Serbian with Croatian and with Bosnian (langtag's reading of CLDR's sh rows, since CLDR
// aliases sh to sr-Latn and comments out its own sr rows), and Malay with
// Indonesian. Each holds both ways, for every spelling and script of the
// language.
func TestNewIntelligiblePairs(t *testing.T) {
	t.Parallel()
	pairs := map[string][2]string{
		"serbian and croatian":            {"sr", "hr"},
		"serbian and bosnian":             {"sr", "bs"},
		"malay and indonesian":            {"ms", "id"},
		"cyrillic serbian and croatian":   {"sr-Cyrl", "hr"},
		"serbo-croatian and bosnian":      {"sh", "bs"},
		"standard malay and indonesian":   {"zsm", "ind"},
		"malaysian malay and indonesian":  {"ms-MY", "id-ID"},
		"montenegrin serbian and bosnian": {"sr-Latn-ME", "bs-BA"},
	}
	for name, p := range pairs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, b := langtag.MustParse(p[0]), langtag.MustParse(p[1])
			for _, d := range [][2]langtag.Tag{{a, b}, {b, a}} {
				if got := langtag.Prefer(d[0]).Compare(d[1]); got != langtag.TierIntelligible {
					t.Errorf("Prefer(%v).Compare(%v) = %v, want %v", d[0], d[1], got, langtag.TierIntelligible)
				}
				if r, ok := langtag.Prefer(d[0]).Reason(d[1]); !ok || r == "" {
					t.Errorf("Prefer(%v).Reason(%v) = %q, %v, want a recorded reason", d[0], d[1], r, ok)
				}
			}
		})
	}
}

// TestUnknownScriptIsNoTierDifference pins that a tag whose script is unknown
// (Zzzz, stated or resolved because CLDR gives the language none) grades as
// the same script as any other tag of its language, so only the region can
// separate them. The zh row reaches the cluster check with a Traditional
// script on one side only, and the cluster counts only Traditional tags.
func TestUnknownScriptIsNoTierDifference(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want, have string
		tier       langtag.Tier
	}{
		"montenegrin against latin":       {"cnr", "cnr-Latn", langtag.TierSameLanguage},
		"montenegrin against cyrillic":    {"cnr", "cnr-Cyrl", langtag.TierSameLanguage},
		"berber against tifinagh":         {"ber", "ber-Tfng", langtag.TierSameLanguage},
		"stated unknown against stated":   {"zh-Zzzz", "zh-Hant", langtag.TierSameLanguage},
		"traditional cluster, one side":   {"zh-Hant-HK", "zh-Zzzz-TW", langtag.TierSameLanguage},
		"unknown script keeps the region": {"en-Zzzz-US", "en-GB", langtag.TierOtherRegion},
		"known scripts still differ":      {"cnr-Latn", "cnr-Cyrl", langtag.TierOtherScript},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, have := langtag.MustParse(tc.want), langtag.MustParse(tc.have)
			if got := langtag.Prefer(want).Compare(have); got != tc.tier {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v", tc.want, tc.have, got, tc.tier)
			}
			if got := langtag.Prefer(have).Compare(want); got != tc.tier {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v", tc.have, tc.want, got, tc.tier)
			}
		})
	}
}

// TestCompareExcludesSharedLiteracy is the load-bearing regression guard: CLDR
// rates every pair below as close because the first language's population is
// broadly literate in the second, which is accurate sociolinguistics and an
// unacceptable substitution rule — a viewer who chose Tamil subtitles did not
// ask for English ones. Tiers 0-3 cannot express a cross-language relation at
// all, so this excludes the whole family by construction.
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

// TestDefaultIsTheBuiltInTable pins the relationship between the package's two
// doors onto the built-in judgments: Prefer binds the table implicitly, Default
// hands it over so a caller can hold it. A Default that returned nothing would
// not fail loudly — a Preference on a nil table grades every pair TierNone — so
// the equivalence is asserted rather than assumed.
func TestDefaultIsTheBuiltInTable(t *testing.T) {
	t.Parallel()
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	if got := langtag.Default().Prefer(ca).Compare(es); got != langtag.TierSharedLiteracy {
		t.Errorf("Default().Prefer(ca).Compare(es) = %v, want %v", got, langtag.TierSharedLiteracy)
	}
	nob, nn := langtag.MustParse("nob"), langtag.MustParse("nn")
	if got := langtag.Default().Prefer(nob).Compare(nn); got != langtag.TierIntelligible {
		t.Errorf("Default().Prefer(nob).Compare(nn) = %v, want %v", got, langtag.TierIntelligible)
	}
	if got, ok := langtag.Default().Prefer(ca).Reason(es); !ok || got == "" {
		t.Errorf("Default().Prefer(ca).Reason(es) = (%q, %v), want a justification and true", got, ok)
	}
}

// TestPreferenceReportsTheWantedLanguage covers what a Preference says about
// itself, which is all a caller holding only the Preference (the build-once
// idiom) has to log. Both answers come back in canonical form, because the Tag
// was canonicalized at construction rather than by either accessor.
func TestPreferenceReportsTheWantedLanguage(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in, canon string
	}{
		"case is normalized":   {"PT-br", "pt-BR"},
		"variant is discarded": {"de-1996", "de"},
		"script is kept":       {"sr-Latn", "sr-Latn"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := langtag.Prefer(langtag.MustParse(tc.in))
			if got := p.Want().String(); got != tc.canon {
				t.Errorf("Prefer(MustParse(%q)).Want().String() = %q, want %q", tc.in, got, tc.canon)
			}
			if got := p.String(); got != tc.canon {
				t.Errorf("Prefer(MustParse(%q)).String() = %q, want %q", tc.in, got, tc.canon)
			}
		})
	}
	t.Run("zero preference names no language", func(t *testing.T) {
		t.Parallel()
		var p langtag.Preference
		if got := p.String(); got != "<none>" {
			t.Errorf("langtag.Preference{}.String() = %q, want %q", got, "<none>")
		}
		if !p.Want().IsZero() {
			t.Errorf("langtag.Preference{}.Want() = %q, want the zero Tag", p.Want())
		}
	})
}

func TestMatch(t *testing.T) {
	t.Parallel()
	nob, nor := langtag.MustParse("nob"), langtag.MustParse("nor")
	nn := langtag.MustParse("nn")
	hant, hans := langtag.MustParse("zh-Hant"), langtag.MustParse("zh-Hans")
	esES, es419 := langtag.MustParse("es-ES"), langtag.MustParse("es-419")

	cases := map[string]struct {
		want, have langtag.Tag
		floor      langtag.Tier
		match      bool
	}{
		"identical floor rejects macrolanguage": {nob, nor, langtag.TierIdentical, false},
		"same-language floor accepts it":        {nob, nor, langtag.TierSameLanguage, true},
		"same-language floor rejects region":    {esES, es419, langtag.TierSameLanguage, false},
		"other-region floor accepts region":     {esES, es419, langtag.TierOtherRegion, true},
		"other-region floor rejects script":     {hans, hant, langtag.TierOtherRegion, false},
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
		"zh-Hant", "cmn", "yue", "ca", "es", "es-419", "es-ES", "hr", "bs", "sr", "ta",
		"en-US", "zh-Hant-HK", "zh-Hant-TW",
	}
	floors := []langtag.Tier{
		langtag.TierIdentical, langtag.TierSameLanguage, langtag.TierOtherRegion,
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
