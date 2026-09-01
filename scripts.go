package langtag

import "slices"

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
// [TierOtherScript] to [TierSameLanguage].
//
// Unlike the cross-language table, this list is derived, not judged: CLDR's
// languageInfo.xml gives a generic same-language cross-script distance of
// 50 and names only a few specific pairs, of which all but one are one-way
// transliteration rows (romanization feeding its native script, not two
// audiences reading each other — the same reason a one-way language
// relation is shared literacy). The one symmetric pair, Serbian, earns the
// promotion twice: CLDR rates it 5, and Serbian schooling teaches both
// scripts.
//
// Deliberately absent: Simplified/Traditional Chinese (CLDR names no
// distance, so it sits at the generic 50), and uz-Latn/uz-Cyrl,
// az-Latn/az-Cyrl (x/text's stale snapshot rates them close but upstream
// CLDR does not name either).
var closeScripts = []CloseScript{
	{
		Language: "sr", A: "Latn", B: "Cyrl",
		Reason: "Serbian is written in both scripts and schooling in Serbia " +
			"teaches both, so a reader handles either without effort.",
		Provenance: "CLDR languageInfo.xml, sr_Latn <-> sr_Cyrl, distance 5, " +
			"symmetric, against a generic same-language cross-script distance of 50.",
	},
}

// CloseScripts returns a copy of the built-in list of script pairs that read as
// one language.
//
// Unlike [Fallbacks], this list is not replaceable. It records where CLDR states
// an explicit symmetric distance for a script pair, which is a fact about the
// data rather than a judgment about people, so a deployment has nothing to
// disagree with. A caller that wants a script difference treated as a barrier
// anyway floors at [TierSameLanguage] and gets that.
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
