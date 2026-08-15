package langtag

// Tier is how far an available language sits from the one that was wanted.
// Lower is closer. A caller picks the highest tier it will accept as a floor.
//
// Tiers 0 through 2 follow from published standards data and hold no opinion.
// [TierSensitive] is the only tier that encodes a judgment, which is why it is
// named for that and why a caller must opt into it.
type Tier uint8

const (
	// TierIdentical means the two tags canonicalize to the same thing. It
	// covers the code systems that spell one language several ways: nor and
	// no, ger and deu, chi and zho, iw and he.
	TierIdentical Tier = iota

	// TierSameLanguage means one language, written the same way, differing at
	// most in region. It covers macrolanguage folding (nob against nor, cmn
	// against zho) and regional variants (es-ES against es-419, pt-BR against
	// pt-PT, en-GB against en-US). Reading is unaffected.
	TierSameLanguage

	// TierOtherScript means one language written in a different script:
	// zh-Hans against zh-Hant, sr-Cyrl against sr-Latn, uz-Latn against
	// uz-Cyrl. Readers of one generally manage the other, with effort.
	TierOtherScript

	// TierSensitive means a different language that a reader of the wanted one
	// can probably use. Every case is a judgment recorded in the fallback
	// table, and several are contested, so this tier is never reached unless a
	// caller asks for it.
	TierSensitive

	// TierNone means no relationship worth acting on. It is also the answer
	// whenever either tag is the zero Tag.
	TierNone
)

// tierNames indexes Tier for String and is the inverse of the ParseTier table.
// The strings are a configuration surface, so they are stable.
var tierNames = [...]string{
	TierIdentical:    "identical",
	TierSameLanguage: "same-language",
	TierOtherScript:  "other-script",
	TierSensitive:    "sensitive",
	TierNone:         "none",
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
	case "sensitive":
		return TierSensitive, true
	default:
		return TierNone, false
	}
}

// normalizeTierName lowercases and folds underscores to hyphens so the
// configuration surface tolerates both spellings.
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
