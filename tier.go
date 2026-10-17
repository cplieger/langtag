package langtag

// Tier is how far an available language sits from the one that was wanted; a
// caller picks the widest tier it accepts as a floor.
//
// The order ranks kinds of claim, narrowest to widest, not reading effort:
// [TierOtherScript], CLDR's generic 50, is farther to read than the curated
// pairs above it. Tiers 0 through 3 follow from standards data.
// [TierIntelligible] and [TierSharedLiteracy] are curated, and the second
// licenses a different language entirely, so it needs its own opt-in.
type Tier uint8

const (
	// TierIdentical means the two tags canonicalize to the same thing. It
	// covers the code systems that spell one language several ways: nor and
	// no, ger and deu, chi and zho, iw and he.
	TierIdentical Tier = iota

	// TierSameLanguage means one language a reader takes in without effort. It
	// covers macrolanguage folding (nob against nor, cmn against zho), regional
	// variants CLDR does not split (es-419 against es-MX, en-US against en-CA,
	// fr-CA against fr-FR), the script pairs whose readers are taught both,
	// which is Serbian Latin against Serbian Cyrillic (see [CloseScripts]), and
	// an unknown script (Zzzz) against any other (cnr against cnr-Latn).
	TierSameLanguage

	// TierOtherRegion means one language in two regional standards: both tags
	// state a region CLDR places, neither contains the other, and they sit on
	// opposite sides of one of CLDR's paradigm clusters (es-419 against es-ES,
	// pt-BR against pt-PT, en-US against en-GB, ar-MA against ar-EG, zh-Hant-HK
	// against zh-Hant-TW). CLDR rates such a pair 5, against 4 or less for two
	// regions on one side; a language with no cluster stays on [TierSameLanguage].
	TierOtherRegion

	// TierOtherScript means one language in a script its readers are NOT
	// generally taught alongside: Simplified against Traditional Chinese,
	// uz-Latn against uz-Cyrl. Readers of one usually manage the other, with
	// effort. The pairs CLDR explicitly rates as close sit on
	// [TierSameLanguage] instead, so what remains here is the set CLDR does not
	// vouch for, which it scores at 50 against 4 to 30 for every curated
	// close-language pair. That is why this tier can be a bigger ask than the
	// two above it.
	TierOtherScript

	// TierIntelligible means two different languages that readers move between
	// because the languages are close: Bokmål and Nynorsk, Danish and
	// Norwegian, Serbian and Croatian, Czech and Slovak, Malay and Indonesian.
	// It runs both ways. These are written-language claims (Danish and
	// Norwegian are far apart aloud), so a caller matching audio should not
	// reach this tier.
	TierIntelligible

	// TierSharedLiteracy means a different, possibly unrelated language that
	// readers of the wanted one are broadly literate in: Catalan readers read
	// Spanish. It runs one way only and is reached only by a floor naming it;
	// the built-in table holds one entry, and [WithFallbacks] adds more.
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
	TierOtherRegion:    "other-region",
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
	case "other-region":
		return TierOtherRegion, true
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

// normalizeTierName lowercases ASCII, folds '_' to '-' and drops spaces and
// tabs. The fold is ASCII-only, like the tag parser's gate: strings.ToLower
// maps U+0130 onto "i" and strings.EqualFold matches U+017F against "s", so a
// Unicode fold would accept impersonating spellings, which fail closed here.
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
