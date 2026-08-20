package langtag_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/cplieger/langtag/v2"
)

// This file pins the charset boundary, which is the property that decides
// whether a toolchain's Unicode tables can move this package's answers.
//
// They cannot, and the reason is structural rather than lucky. RFC 5646 §2.1
// defines every BCP 47 subtag as ASCII alphanumerics, so the grammar itself
// admits nothing else. golang.org/x/text enforces that with a byte-level gate
// (internal/language.isAlphaNum) applied to each hyphen-delimited token as it is
// scanned, BEFORE any case folding, and its folding is byte-level ASCII
// arithmetic (internal/language.(*scanner).toLower) rather than a Unicode fold.
// Tier names take the same route through this package's own ASCII-only
// normaliseTierName. So no unicode.RangeTable, no SimpleFold orbit and no
// ToLower mapping is consulted on any path a caller can reach.
//
// That matters because the alternative shape is a live hazard elsewhere: a
// [strings.EqualFold] used as an IDENTITY check accepts a non-ASCII rune that
// folds onto an ASCII one, which is how "localhoſt" once classified as loopback
// in a sibling library. Subtag matching is exactly an identity check, so these
// tests assert the refusal at each impersonation rather than trusting the
// grammar.
//
// Measured across a real go1.26.7 and a real go1.27.0 toolchain (Unicode 15.0.0
// against 17.0.0): Parse's accept set, canonical output, Language, Script and
// every Compare tier are byte-identical, while strings.EqualFold's answer
// changed for three rune pairs in the same interval. The delta between those two
// facts is the whole value of the ASCII gate.

// asciiFoldSpoofs pairs a real subtag with a non-ASCII string that a Unicode
// fold or a Unicode lowercase maps onto it. Each spoof must be REFUSED even
// though a Unicode-aware comparison would call it equal.
//
// The impersonating runes are the complete set for ASCII targets, measured over
// all 1,114,112 code points and IDENTICAL on Unicode 15 and 17: U+017F LATIN
// SMALL LETTER LONG S folds onto S/s, U+212A KELVIN SIGN folds onto K/k, and
// U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE lowercases to i. Unicode 17 grew
// the fold tables by 116 runes without adding a single new ASCII impersonator,
// so this list is stable across the bump rather than merely current.
var asciiFoldSpoofs = map[string]struct {
	spoof  string // the non-ASCII candidate
	honest string // the real subtag it impersonates
}{
	"long s for slovak":       {"\u017Fk", "sk"},
	"long s for czech":        {"c\u017F", "cs"},
	"long s for bosnian":      {"b\u017F", "bs"},
	"long s for spanish":      {"e\u017F", "es"},
	"long s for serbian":      {"\u017Fr", "sr"},
	"long s for icelandic":    {"i\u017F", "is"},
	"kelvin sign for kazakh":  {"\u212Ak", "kk"},
	"kelvin sign for kashmir": {"\u212As", "ks"},
	"dotted I for indonesian": {"\u0130d", "id"},
	"dotted I for sinhala":    {"s\u0130", "si"},
}

// unicode17Changed names every rune whose classification or case mapping moved
// between Unicode 15 and Unicode 17, measured rather than read. Each one is a
// class of change, and none may reach an accepted tag.
var unicode17Changed = map[string]rune{
	// SimpleFold gained these three pairs; both members of each pair already
	// existed in Unicode 15, so a corpus can contain them today. This is the
	// only class that changes strings.EqualFold's answer for pre-existing input.
	"fold pair a, member 1": 0x0390,
	"fold pair a, member 2": 0x1FD3,
	"fold pair b, member 1": 0x03B0,
	"fold pair b, member 2": 0x1FE3,
	"fold pair c, member 1": 0xFB05,
	"fold pair c, member 2": 0xFB06,

	// The dangerous category flip: an ALREADY-ASSIGNED rune leaves Ll for Lo, so
	// unicode.IsLower goes true -> false and unicode.Is(unicode.LC, ·) with it.
	// The usual reassurance for this release is that every added rune was
	// unassigned in Unicode 15 and therefore absent from any existing corpus;
	// this rune defeats that argument, which is why it is named here.
	"Ll to Lo, already assigned": 0x0295,

	// The other category flip, in the opposite direction from every other
	// change: Mn to Mc.
	"Mn to Mc": 0x1171E,

	// Four already-assigned runes gained an uppercase mapping, so ToUpper
	// changes its answer for pre-existing input. Absent from the release notes.
	"gained a ToUpper mapping": 0x019B,

	// A newly assigned letter: unicode.IsLetter answers false on Unicode 15 and
	// true on Unicode 17. IsLetter grew by 9,568 runes, and this stands for all
	// of them.
	"newly assigned letter": 0x105C0,
}

// TestParseRefusesEveryNonASCIIRune is the load-bearing test in this file: it
// makes the ASCII-only accept set a checked contract of this package rather than
// an inherited property of its dependency.
//
// It sweeps every code point above ASCII that carries a letter, a digit, or any
// case or fold mapping — 146,465 runes on Unicode 17 — through five tag shapes
// covering the language, script and region positions. Not one may be accepted.
// A future change that replaced the byte gate with a Unicode-aware one, or that
// began folding with strings.EqualFold, fails here immediately.
func TestParseRefusesEveryNonASCIIRune(t *testing.T) {
	t.Parallel()
	probed, accepted := 0, 0
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
			unicode.SimpleFold(r) == r && unicode.ToLower(r) == r && unicode.ToUpper(r) == r {
			continue
		}
		probed++
		for _, shape := range []string{
			string(r) + "n",           // two-letter primary subtag
			"e" + string(r),           // two-letter primary subtag, second position
			string(r) + string(r),     // both positions
			"en-" + string(r) + "atn", // script position
			"en-" + string(r) + "B",   // region position
		} {
			got, ok := langtag.Parse(shape)
			if ok {
				accepted++
				if accepted <= 10 {
					t.Errorf("Parse(%+q) = (%q, true), want ok=false; BCP 47 subtags are ASCII alphanumerics (RFC 5646 §2.1)",
						shape, got.String())
				}
			}
		}
	}
	if probed == 0 {
		t.Fatal("swept no runes; the filter is wrong and this test is vacuous")
	}
	if accepted != 0 {
		t.Errorf("Parse accepted %d of %d non-ASCII candidate tags, want 0", accepted, probed*5)
	}
}

// TestParseRefusesASCIIFoldSpoofs is the per-site form of the same property,
// and the one that explains why it matters.
//
// Each case asserts two things together: strings.EqualFold calls the spoof equal
// to a real subtag, and Parse refuses it anyway. Read as a pair they show what
// the ASCII gate buys — had this package folded subtags with EqualFold, every
// one of these would have been accepted as the language it impersonates.
func TestParseRefusesASCIIFoldSpoofs(t *testing.T) {
	t.Parallel()
	for name, tc := range asciiFoldSpoofs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The honest subtag must really parse, or the case proves nothing.
			honest, ok := langtag.Parse(tc.honest)
			if !ok {
				t.Fatalf("Parse(%q) = ok=false; the honest side of this case must be a real subtag", tc.honest)
			}
			// A Unicode-aware comparison cannot tell the two apart. At least one
			// of the two spellings must collide, or the spoof is not a spoof.
			foldEqual := strings.EqualFold(tc.spoof, tc.honest) ||
				strings.ToLower(tc.spoof) == tc.honest
			if !foldEqual {
				t.Fatalf("neither EqualFold(%+q, %q) nor ToLower collision holds; this case does not test an impersonation",
					tc.spoof, tc.honest)
			}
			// And Parse must refuse it regardless.
			got, ok := langtag.Parse(tc.spoof)
			if ok {
				t.Fatalf("Parse(%+q) = (%q, true), want ok=false; a Unicode fold must not reach an ASCII subtag",
					tc.spoof, got.String())
			}
			if !got.IsZero() {
				t.Errorf("Parse(%+q) returned non-zero Tag %q on refusal, want the zero Tag", tc.spoof, got.String())
			}
			// The refusal must hold all the way through comparison: a spoof can
			// never stand in for the language it impersonates, at any floor.
			for _, floor := range []langtag.Tier{
				langtag.TierIdentical, langtag.TierSameLanguage, langtag.TierOtherScript,
				langtag.TierIntelligible, langtag.TierSharedLiteracy,
			} {
				if langtag.Prefer(honest).Match(got, floor) {
					t.Errorf("Prefer(%q).Match(spoof %+q, %v) = true, want false", tc.honest, tc.spoof, floor)
				}
				if langtag.Prefer(got).Match(honest, floor) {
					t.Errorf("Prefer(spoof %+q).Match(%q, %v) = true, want false", tc.spoof, tc.honest, floor)
				}
			}
		})
	}
}

// TestParseRefusesUnicode17ChangedRunes pins the specific runes whose
// classification or case mapping moved between Unicode 15 and Unicode 17, one
// per class of change. See unicode17Changed for what each class is.
func TestParseRefusesUnicode17ChangedRunes(t *testing.T) {
	t.Parallel()
	for name, r := range unicode17Changed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, shape := range []string{
				string(r), string(r) + "n", "e" + string(r), string(r) + string(r),
				"en-" + string(r) + "atn", "en-" + string(r) + "B",
				"ca-" + string(r) + "alencia", "en-u-" + string(r) + "a",
			} {
				got, ok := langtag.Parse(shape)
				if ok {
					t.Errorf("Parse(%+q) = (%q, true), want ok=false (U+%04X)", shape, got.String(), r)
				}
			}
		})
	}
}

// TestCanonicalFormIsASCII covers the output side of the same boundary.
//
// Tag.String is documented as usable for persistence and Tag.Language as a map
// key, so both must be free of any rune a case-insensitive comparison could
// collide. Any non-ASCII byte in either would put a consumer's own lookup back
// on the wrong side of the fold tables even though this package stayed clean.
func TestCanonicalFormIsASCII(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"ger", "deu", "nob", "nor", "no", "nb", "nn", "zh", "cmn", "zh-Hans",
		"ZH-HANT", "sr-Latn", "SR-CYRL", "es-419", "pt-BR", "en-gb", "  de  ",
		"de_DE", "iw", "he", "tl", "fil", "arb", "swh", "yue", "arz", "cy",
		"en-GB-oed", "i-klingon", "zh-min-nan", "ca-valencia", "de-1996",
		"en-US-u-va-posix", "AA-u-0A-0A-u-00-00",
	}
	for _, raw := range inputs {
		tag, ok := langtag.Parse(raw)
		if !ok {
			continue
		}
		for _, out := range []struct{ what, s string }{
			{"String", tag.String()},
			{"Language", tag.Language()},
			{"Script", tag.Script()},
		} {
			for i := range len(out.s) {
				if out.s[i] >= 0x80 {
					t.Errorf("Parse(%q).%s() = %+q, which carries a non-ASCII byte at %d; a consumer keying on it would be exposed to the fold tables",
						raw, out.what, out.s, i)
					break
				}
			}
		}
	}
}

// TestParseTierFoldsASCIIOnly covers the package's own case folding, which is
// the one fold it does not delegate.
//
// ParseTier reads operator-supplied configuration, so it lowercases before
// matching. It does so with ASCII byte arithmetic, which means a Unicode
// spelling that a Unicode-aware fold would accept fails closed instead — and
// failing closed here is the safe direction, because ParseTier's failure value
// is TierNone, a floor that matches nothing.
func TestParseTierFoldsASCIIOnly(t *testing.T) {
	t.Parallel()
	t.Run("ascii case and separator forms are accepted", func(t *testing.T) {
		t.Parallel()
		for _, s := range []string{
			"identical", "IDENTICAL", "Identical", "same-language", "SAME-LANGUAGE",
			"same_language", " same-language ", "other-script", "intelligible",
			"shared-literacy", "shared_literacy",
		} {
			if _, ok := langtag.ParseTier(s); !ok {
				t.Errorf("ParseTier(%q) ok = false, want true", s)
			}
		}
	})
	t.Run("unicode spellings fail closed", func(t *testing.T) {
		t.Parallel()
		// Each of these is a tier name with one ASCII letter replaced by a rune
		// that a Unicode fold or lowercase maps onto it. All must be refused,
		// and the refusal value must be the floor that matches nothing.
		for _, s := range []string{
			"\u0130DENTICAL",     // U+0130, lowercases to "i"
			"IDENT\u0130CAL",     //
			"\u017Fame-language", // U+017F, folds onto "s"
			"same-language\u017F",
			"\u212Aidentical", // U+212A, folds onto "k"
			"other-\u017Fcript",
			"\u0131dentical", // dotless i, folds onto nothing ASCII
		} {
			tier, ok := langtag.ParseTier(s)
			if ok {
				t.Errorf("ParseTier(%+q) ok = true, want false; the config fold is ASCII-only by design", s)
			}
			if tier != langtag.TierNone {
				t.Errorf("ParseTier(%+q) = %v on refusal, want %v", s, tier, langtag.TierNone)
			}
			// The refusal value must not widen anything.
			if langtag.Prefer(langtag.MustParse("ta")).Match(langtag.MustParse("en"), tier) {
				t.Errorf("the floor ParseTier(%+q) returned matches an unrelated pair, want a floor that matches nothing", s)
			}
		}
	})
}
