package langtag

// Tier is how far an available language sits from the one that was wanted. A
// caller picks the highest tier it will accept as a floor.
//
// The order is by KIND OF LICENSE, from narrowest to widest: the same tag, then
// the same language, then the same language in another script, then a close
// language, then a language readers merely happen to be schooled in. It is
// deliberately NOT a reading-difficulty scale, and reading difficulty is not
// monotonic along it. [TierOtherScript] is where that shows: what remains on it
// is the script pairs CLDR does not explicitly rate, which it scores at 50,
// farther than any pair on the two tiers above. A caller serving Chinese content
// should weigh that before accepting TierOtherScript.
//
// Tiers 0 through 2 follow from published standards data and hold no opinion.
// The two tiers above them are curated judgments, and they are separated
// because they make different kinds of claim: [TierIntelligible] says two
// languages are interchangeable, while [TierSharedLiteracy] says only that
// readers of one are, as a population, literate in the other. The second is a
// much weaker claim and the substitution it licenses is a different language
// entirely, so reaching it takes its own opt-in.
type Tier uint8

const (
	// TierIdentical means the two tags canonicalize to the same thing. It
	// covers the code systems that spell one language several ways: nor and
	// no, ger and deu, chi and zho, iw and he.
	TierIdentical Tier = iota

	// TierSameLanguage means one language a reader takes in without effort. It
	// covers macrolanguage folding (nob against nor, cmn against zho), regional
	// variants (es-ES against es-419, pt-BR against pt-PT, en-GB against en-US),
	// and the script pairs whose readers are taught both, which is Serbian
	// Latin against Serbian Cyrillic (see [CloseScripts]).
	TierSameLanguage

	// TierOtherScript means one language in a script its readers are NOT
	// generally taught alongside: Simplified against Traditional Chinese,
	// uz-Latn against uz-Cyrl. Readers of one usually manage the other, with
	// effort. The pairs CLDR explicitly rates as close sit on the tier below
	// instead, so what remains here is the set CLDR does not vouch for, which
	// it scores at 50 against 4 to 20 for every curated close-language pair.
	// That is why this tier can be a bigger ask than the two above it.
	TierOtherScript

	// TierIntelligible means two different languages that readers move between
	// because the languages themselves are close: Bokmål and Nynorsk, Danish
	// and Norwegian, Croatian and Bosnian, Czech and Slovak. The relationship
	// runs both ways, which is the mark of interchangeability rather than of
	// one population's schooling.
	//
	// These are written-language claims. Two languages readable to each other
	// on the page are not necessarily comprehensible to each other aloud, and
	// Danish against Norwegian is exactly that case. A caller matching audio
	// should not reach this tier.
	TierIntelligible

	// TierSharedLiteracy means a different, not necessarily related language
	// that readers of the wanted one can use because they are broadly literate
	// in it. Catalan readers read Spanish. The relationship runs one way only,
	// and the two languages may be unrelated.
	//
	// This tier exists so the claim can be made honestly and separately rather
	// than smuggled in beside interchangeability. It is never reached unless a
	// caller asks for it by name, and the table ships one entry: extending it
	// to the rest of the family is a decision for whoever runs the software,
	// through [WithFallbacks].
	TierSharedLiteracy

	// TierNone means no relationship worth acting on. It is also the answer
	// whenever either tag is the zero Tag.
	TierNone
)

// The configuration spellings of the two curated tiers. Named because
// Tier.String, ParseTier and Kind.String must not drift apart.
const (
	nameIntelligible   = "intelligible"
	nameSharedLiteracy = "shared-literacy"
)

// tierNames indexes Tier for String and is the inverse of the ParseTier table.
// The strings are a configuration surface, so they are stable.
var tierNames = [...]string{
	TierIdentical:      "identical",
	TierSameLanguage:   "same-language",
	TierOtherScript:    "other-script",
	TierIntelligible:   nameIntelligible,
	TierSharedLiteracy: nameSharedLiteracy,
	TierNone:           "none",
}

// String returns the configuration spelling of the tier.
func (t Tier) String() string {
	if int(t) >= len(tierNames) {
		return "invalid"
	}
	return tierNames[t]
}

// ParseTier maps a configuration string onto a Tier. It accepts the spellings
// String produces, in any letter case, plus the underscore form so that an
// operator who writes same_language is not punished for it. ok is false for
// anything else, and for "none", which is not a usable floor: a floor of
// TierNone would accept every language as a substitute for every other.
func ParseTier(s string) (Tier, bool) {
	switch normalizeTierName(s) {
	case "identical":
		return TierIdentical, true
	case "same-language":
		return TierSameLanguage, true
	case "other-script":
		return TierOtherScript, true
	case nameIntelligible:
		return TierIntelligible, true
	case nameSharedLiteracy:
		return TierSharedLiteracy, true
	default:
		return TierNone, false
	}
}

// normalizeTierName lowercases and folds underscores to hyphens so the
// configuration surface tolerates both spellings.
//
// The fold is ASCII byte arithmetic rather than strings.ToLower, matching the
// tag parser's own ASCII-only gate. The tier names are ASCII by construction, so
// a Unicode fold could only ever widen what is accepted — and it would widen it
// onto exactly the impersonation shape the tag side refuses, since
// strings.ToLower maps U+0130 onto "i" and strings.EqualFold matches U+017F
// against "s". A Unicode spelling therefore fails closed here, which is the safe
// direction: ParseTier's failure value is TierNone, a floor that matches nothing.
func normalizeTierName(s string) string {
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case c == '_':
			out = append(out, '-')
		case c == ' ' || c == '\t':
			// Drop surrounding and internal whitespace rather than failing on it.
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
