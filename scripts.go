package langtag

import (
	"cmp"
	"slices"
	"strings"
)

// unknownScript is ISO 15924's code for an undetermined script, what a tag
// resolves to when it states no script and the table holds no row for it.
const unknownScript = "Zzzz"

// languageScript and regionScript are the rows of scripttable.go, generated
// from the pinned CLDR likelySubtags.xml. languageScripts is sorted by
// language and regionScripts by language then region; the lookups below
// binary-search them, so an unsorted table answers wrongly
// (TestScriptTableSorted).
type languageScript struct{ language, script string }

type regionScript struct{ language, region, script string }

// resolveScript is CLDR's likely-subtags lookup restricted to the script: the
// first row keyed <lang>_<region>, <macro>_<region>, <lang>, <macro>, else
// unknown. lang is the tag's own primary subtag and macro its macrolanguage,
// so cmn-MY and zh-MY agree though CLDR keys only zh.
func resolveScript(lang, macro, region string) string {
	if region != "" {
		if s, ok := regionRow(lang, region); ok {
			return s
		}
		if s, ok := regionRow(macro, region); ok {
			return s
		}
	}
	if s, ok := languageRow(lang); ok {
		return s
	}
	if s, ok := languageRow(macro); ok {
		return s
	}
	return unknownScript
}

func languageRow(lang string) (string, bool) {
	i, ok := slices.BinarySearchFunc(languageScripts, lang, func(r languageScript, l string) int {
		return strings.Compare(r.language, l)
	})
	if !ok {
		return "", false
	}
	return languageScripts[i].script, true
}

func regionRow(lang, region string) (string, bool) {
	i, ok := slices.BinarySearchFunc(regionScripts, regionScript{language: lang, region: region}, compareRegionRows)
	if !ok {
		return "", false
	}
	return regionScripts[i].script, true
}

// hasRegionRows reports whether the table holds any region row for lang,
// meaning CLDR writes lang in a script that varies by region.
func hasRegionRows(lang string) bool {
	i, _ := slices.BinarySearchFunc(regionScripts, regionScript{language: lang}, compareRegionRows)
	return i < len(regionScripts) && regionScripts[i].language == lang
}

func compareRegionRows(a, b regionScript) int {
	return cmp.Or(strings.Compare(a.language, b.language), strings.Compare(a.region, b.region))
}

// CloseScript is a pair of scripts that one language's readers move between
// freely, so a difference between them is not a barrier the way a script
// difference usually is.
//
// This is a narrow escape hatch from the structural rule that a script
// difference is farther than a region difference. That rule is right in general
// and wrong for a handful of languages written in two scripts whose readers are
// taught both.
type CloseScript struct {
	// Language is the primary subtag after macrolanguage folding.
	Language string
	// A and B are the ISO 15924 script codes, in either order.
	A, B string
	// Reason states why readers move between them.
	Reason string
	// Provenance records where the claim can be re-checked.
	Provenance string
}

// closeScripts promotes a same-language script difference from
// [TierOtherScript] to [TierSameLanguage]. It is derived from CLDR's
// languageInfo.xml, not judged: of the few pairs it names against the generic
// cross-script distance of 50, all but Serbian's are one-way transliteration
// rows. Serbian is symmetric at 5, and its schooling teaches both scripts.
// Absent on purpose: Simplified/Traditional Chinese (no named distance) and
// uz or az Latn/Cyrl (close only in x/text's stale snapshot).
var closeScripts = []CloseScript{
	{
		Language: "sr", A: scriptLatn, B: scriptCyrl,
		Reason: "Serbian is written in both scripts and schooling in Serbia " +
			"teaches both, so a reader handles either without effort.",
		Provenance: "CLDR languageInfo.xml, sr_Latn <-> sr_Cyrl, distance 5, " +
			"symmetric, against a generic same-language cross-script distance of 50.",
	},
}

// CloseScripts returns a copy of the built-in, non-replaceable list of script
// pairs CLDR rates as one language. A close pair matches at
// [TierSameLanguage]; to treat a script difference as a barrier, compare
// [Tag.Script] or floor at [TierIdentical], which also refuses region
// differences.
func CloseScripts() []CloseScript {
	return slices.Clone(closeScripts)
}

// scriptsReadAsOne reports whether a difference between two scripts of one
// language is one its readers move across freely.
func scriptsReadAsOne(language, scriptA, scriptB string) bool {
	for _, cs := range closeScripts {
		if cs.Language != language {
			continue
		}
		if (cs.A == scriptA && cs.B == scriptB) || (cs.A == scriptB && cs.B == scriptA) {
			return true
		}
	}
	return false
}
