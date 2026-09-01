package langtag

import (
	"strings"

	"golang.org/x/text/language"
)

// Tag is a canonical BCP 47 language tag that names an actual language.
//
// The zero Tag is invalid: it is what [Parse] returns for input that names no
// language, and it matches nothing at any tier, including another zero Tag.
// Two tracks both tagged "undetermined" are not known to share a language.
//
// Tag holds only strings, so it is comparable, copyable and carries no
// reference to the parsing library in its public shape.
type Tag struct {
	// canon is the canonical BCP 47 form, empty for the zero Tag. It decides
	// TierIdentical and is what String reports.
	canon string
	// macroBase is the primary language subtag after macrolanguage folding, so
	// that nb and no share "no" and cmn and zh share "zh". It decides whether
	// two tags name the same language.
	macroBase string
	// script is the ISO 15924 script, resolved through CLDR's default-script
	// data when the tag does not state one, so that a bare "zh" compares equal
	// to "zh-Hans" and a bare "sr" to "sr-Cyrl".
	script string
}

// nonLanguages are well-formed subtags that name no specific language. They
// parse successfully and must still be rejected: a track tagged with one of
// them tells us nothing about what a viewer would understand.
var nonLanguages = map[string]struct{}{
	"und": {}, // undetermined
	"zxx": {}, // no linguistic content
	"mul": {}, // multiple languages
	"mis": {}, // uncoded languages
}

// Parse canonicalizes one raw language identifier from an untrusted source.
//
// It accepts ISO 639-1, ISO 639-2 (bibliographic or terminological), ISO 639-3
// and BCP 47 tags, in any ASCII letter case, with surrounding whitespace. ok is
// false, and the returned Tag is the zero Tag, for the empty string, for the
// placeholder subtags that name no language (und, zxx, mul, mis), for
// private-use subtags (qaa through qtz), and for anything the IANA Language
// Subtag Registry does not know.
//
// The accept set is ASCII alphanumerics only, per RFC 5646 §2.1's grammar, so
// case folding here is ASCII byte arithmetic, never a Unicode fold — verified
// byte-identical across Unicode 15 and 17 in unicode_test.go, which also
// asserts that a rune a Unicode fold maps onto ASCII cannot impersonate a
// subtag ("İd" is not Indonesian, "ſk" is not Slovak).
//
// Parse never reports an error value; there is no recovery from a malformed
// tag beyond ignoring it.
func Parse(raw string) (Tag, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Tag{}, false
	}
	t, err := language.Parse(s)
	if err != nil {
		return Tag{}, false
	}
	// Raw reports the subtags as written. Base would instead infer a language
	// for a placeholder tag: language.Parse("und").Base() answers "en", which
	// would silently turn an untagged track into English.
	rawBase, rawScript, rawRegion := t.Raw()
	base := rawBase.String()
	if _, bad := nonLanguages[base]; bad {
		return Tag{}, false
	}
	if isPrivateUse(base) {
		return Tag{}, false
	}

	// Rebuild the tag from exactly the language, script and region, discarding
	// variant and extension subtags (-u-, -t-, -x-, and forms like ca-valencia
	// or de-1996). None of them changes which language a track is in, and the
	// library's canonical form for them is not a fixed point: a fuzz run found
	// that Parse("AA-u-0A-0A-u-00-00") yields a string that re-parses to
	// something shorter. The canonical form is used as a persistence key, so it
	// has to survive a round trip.
	composed, err := language.Compose(rawBase, rawScript, rawRegion)
	if err != nil {
		return Tag{}, false
	}

	macro, err := language.Macro.Canonicalize(composed)
	if err != nil {
		// Canonicalization of an already-parsed tag does not fail in practice.
		// Fall back to the parsed tag rather than rejecting a valid language.
		macro = composed
	}
	macroBase, _, _ := macro.Raw()

	// Tag.Script resolves an unstated script through CLDR's default-script
	// data rather than reporting nothing, which is what makes a bare "zh" and
	// an explicit "zh-Hans" one language instead of a script difference. The
	// confidence it also returns distinguishes stated from inferred, and is
	// deliberately ignored: for deciding whether two tags are written the same
	// way, an inferred Latn is as good as a stated one.
	script, _ := composed.Script()

	return Tag{
		canon:     composed.String(),
		macroBase: macroBase.String(),
		script:    script.String(),
	}, true
}

// isPrivateUse reports whether a primary language subtag falls in the
// BCP 47 private-use range qaa through qtz. Such a tag is meaningful only
// inside the system that assigned it, so it can never justify a substitution.
func isPrivateUse(base string) bool {
	return len(base) == 3 && base[0] == 'q' && base[1] >= 'a' && base[1] <= 't'
}

// MustParse is [Parse] for identifiers fixed at compile time. It panics when
// raw does not name a language, so it belongs in package-level tables and
// tests, never on a path that handles input.
func MustParse(raw string) Tag {
	t, ok := Parse(raw)
	if !ok {
		panic("langtag: MustParse(" + raw + "): not a language tag")
	}
	return t
}

// Valid reports whether raw names a language. Intended for validating
// configuration at load time, so that a typo fails loudly at startup instead
// of silently matching nothing forever.
func Valid(raw string) bool {
	_, ok := Parse(raw)
	return ok
}

// String returns the canonical BCP 47 form, or the empty string for the zero
// Tag.
func (t Tag) String() string { return t.canon }

// IsZero reports whether t names no language. Every comparison involving a
// zero Tag yields [TierNone].
func (t Tag) IsZero() bool { return t.canon == "" }

// Language returns the primary language subtag after macrolanguage folding:
// "no" for both nob and nor, "zh" for both cmn and zho. Two tags share a
// language exactly when this value matches. It is the right key for a
// per-language cache or a learned-preference map, because it does not
// fragment across the code systems that name one language several ways.
func (t Tag) Language() string { return t.macroBase }

// Script returns the ISO 15924 script code, resolved to CLDR's default for
// the language when the tag itself does not state one.
func (t Tag) Script() string { return t.script }
