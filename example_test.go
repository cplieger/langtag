package langtag_test

import (
	"fmt"

	"github.com/cplieger/langtag/v2"
)

// One language spelled two ways is one language.
func ExampleParse() {
	for _, raw := range []string{"ger", "deu", "de", "nob", "nor"} {
		tag, _ := langtag.Parse(raw)
		fmt.Printf("%-4s -> tag %-3s language %s\n", raw, tag, tag.Language())
	}
	// Output:
	// ger  -> tag de  language de
	// deu  -> tag de  language de
	// de   -> tag de  language de
	// nob  -> tag nb  language no
	// nor  -> tag no  language no
}

// Compare grades the distance rather than answering yes or no.
func ExamplePreference_Compare() {
	p := langtag.Prefer(langtag.MustParse("nob"))
	for _, raw := range []string{"nob", "nor", "nn", "sv"} {
		have := langtag.MustParse(raw)
		fmt.Printf("a viewer wanting nob, offered %-4s: %s\n", raw, p.Compare(have))
	}
	// Output:
	// a viewer wanting nob, offered nob : identical
	// a viewer wanting nob, offered nor : same-language
	// a viewer wanting nob, offered nn  : intelligible
	// a viewer wanting nob, offered sv  : none
}

// Cross-language relationships are directed, because shared literacy usually
// runs one way: Catalan readers read Spanish, and Spanish readers do not read
// Catalan. The Preference carries the direction: whoever was named in Prefer
// is the one doing the choosing.
func ExamplePreference_Compare_directed() {
	ca, es := langtag.MustParse("ca"), langtag.MustParse("es")
	fmt.Println("wanting ca, offered es:", langtag.Prefer(ca).Compare(es))
	fmt.Println("wanting es, offered ca:", langtag.Prefer(es).Compare(ca))
	// Output:
	// wanting ca, offered es: shared-literacy
	// wanting es, offered ca: none
}

// Best selects from a caller's own type and returns every candidate at the
// closest tier, leaving the final choice to whatever the caller ranks by.
func ExampleBest() {
	type subtitle struct {
		file  string
		codec string
		lang  langtag.Tag
	}
	available := []subtitle{
		{"ep2.eng.srt", "srt", langtag.MustParse("eng")},
		{"ep2.nor.ass", "ass", langtag.MustParse("nor")},
		{"ep2.nor.srt", "srt", langtag.MustParse("nor")},
		{"ep2.swe.srt", "srt", langtag.MustParse("swe")},
	}
	want := langtag.Prefer(langtag.MustParse("nob")) // the viewer chose Bokmål on episode 1

	matches, tier, ok := langtag.Best(want, available,
		func(s subtitle) langtag.Tag { return s.lang },
		langtag.TierSameLanguage)
	if !ok {
		fmt.Println("no usable subtitle")
		return
	}
	fmt.Printf("matched at tier %s:\n", tier)
	for _, m := range matches {
		fmt.Printf("  %s (%s)\n", m.file, m.codec)
	}
	// Output:
	// matched at tier same-language:
	//   ep2.nor.ass (ass)
	//   ep2.nor.srt (srt)
}

// A floor keeps a caller from reaching further than it intends.
func ExamplePreference_Match() {
	p := langtag.Prefer(langtag.MustParse("zh-Hans"))
	have := langtag.MustParse("zh-Hant")
	fmt.Println("at same-language:", p.Match(have, langtag.TierSameLanguage))
	fmt.Println("at other-script: ", p.Match(have, langtag.TierOtherScript))
	// Output:
	// at same-language: false
	// at other-script:  true
}

// Reason explains a cross-language substitution, so a surprised user can be
// told why.
func ExamplePreference_Reason() {
	reason, ok := langtag.Prefer(langtag.MustParse("nb")).Reason(langtag.MustParse("da"))
	fmt.Println(ok)
	fmt.Println(reason)
	// Output:
	// true
	// Written Bokmål derives from Danish and the two remain close on the page. This is a written-language claim only; the spoken languages are much further apart.
}

// A deployment that disagrees with a shipped judgment replaces the table
// instead of forking the package. Passing nil disables tier 3 outright.
func ExampleWithFallbacks() {
	strict := langtag.WithFallbacks(nil)
	nb, nn := langtag.MustParse("nb"), langtag.MustParse("nn")
	fmt.Println("built-in:", langtag.Prefer(nb).Compare(nn))
	fmt.Println("no table:", strict.Prefer(nb).Compare(nn))
	// Output:
	// built-in: intelligible
	// no table: none
}
