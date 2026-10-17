package langtag

import (
	"strings"

	"github.com/cplieger/langtag/v3/internal/multiscript"
	"golang.org/x/text/language"
)

// Tag is a canonical BCP 47 language tag that names an actual language.
//
// The zero Tag is invalid: it is what [Parse] returns for input that names no
// language, and it matches nothing at any tier, including another zero Tag.
// Two tracks both tagged "undetermined" are not known to share a language.
//
// Tag holds only strings and values derived from canon, so == is canonical
// equality and its public shape carries no reference to the parsing library.
type Tag struct {
	// canon is the canonical BCP 47 form, empty for the zero Tag. It decides
	// TierIdentical and is what String reports.
	canon string
	// macroBase is the primary language subtag after macrolanguage folding, so
	// that nb and no share "no" and cmn and zh share "zh". It decides whether
	// two tags name the same language.
	macroBase string
	// script is the ISO 15924 script: the stated one, else the generated CLDR
	// table's (scripts.go resolveScript), so that a bare "zh" compares equal to
	// "zh-Hans" and a bare "sr" to "sr-Cyrl".
	script string
	// scriptAt and regionAt index the stated script and region subtags in
	// canon, 0 when absent (index 0 is the language). Offsets rather than
	// strings keep Tag small enough to pass by value; the region is canon's
	// last subtag because Parse composes only language, script and region.
	scriptAt, regionAt uint8
	// multiScript reports that the table holds region rows for the tag's
	// subtag or its macroBase.
	multiScript bool
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

// Parse canonicalizes one raw language identifier from an untrusted source:
// ISO 639-1, ISO 639-2 (bibliographic or terminological), ISO 639-3 or BCP 47,
// in any ASCII letter case, with surrounding whitespace. ok is false, with the
// zero Tag, for the empty string, the subtags naming no language (und, zxx,
// mul, mis), private-use subtags (qaa through qtz) and anything the IANA
// Language Subtag Registry does not know. Case folding is ASCII only, per
// RFC 5646 §2.1, so a rune a Unicode fold maps onto ASCII cannot impersonate
// a subtag ("İd" is not Indonesian, "ſk" is not Slovak).
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

	own, macroBase := multiscript.Subtags(composed)

	// The script and region come from the composed tag, never from x/text's
	// likely-subtags data, which lags CLDR (zh-MY resolves to Hant there): an
	// unstated script is read from the pinned table instead.
	_, script, region := composed.Raw()
	out := Tag{
		canon:     composed.String(),
		macroBase: macroBase,
	}
	// x/text reports an absent subtag as the zero value, whose String is a
	// real code ("Zzzz", "ZZ"), so absence is tested on the value. canon is
	// lang[-Script][-Region], which fixes both offsets.
	if script != (language.Script{}) {
		out.scriptAt = uint8(len(own) + 1) //nolint:gosec // a language subtag is at most 8 bytes (RFC 5646)
	}
	if region != (language.Region{}) {
		out.regionAt = uint8(strings.LastIndexByte(out.canon, '-') + 1) //nolint:gosec // canon, lang-Script-Region, is at most 17 bytes
	}
	out.script = out.StatedScript()
	if out.script == "" {
		out.script = resolveScript(own, out.macroBase, out.Region())
	}
	out.multiScript = multiscript.Holds(hasRegionRows, own, out.macroBase)
	return out, true
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

// Script returns the ISO 15924 script code. When the tag states none it is
// CLDR's likely script for the tag's language and region, read from a table
// generated from a pinned CLDR release, or "Zzzz" when CLDR gives none. It is
// "" for the zero Tag.
func (t Tag) Script() string { return t.script }

// StatedScript returns the script subtag the canonical form states, or ""
// when it states none.
func (t Tag) StatedScript() string {
	if t.scriptAt == 0 {
		return ""
	}
	return t.canon[t.scriptAt : t.scriptAt+4]
}

// Region returns the region subtag the canonical form states ("BR", "419"),
// or "" when it states none.
func (t Tag) Region() string {
	if t.regionAt == 0 {
		return ""
	}
	return t.canon[t.regionAt:]
}

// MultiScript reports whether CLDR writes the tag's language in a script
// that varies by region: the script table holds a region row for the tag's
// own subtag or for its [Tag.Language] (kmr through ku). For such a language
// a tag stating a region but no script leaves the script to that region.
func (t Tag) MultiScript() bool { return t.multiScript }
