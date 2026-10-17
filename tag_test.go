package langtag_test

import (
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3"
)

func TestParse(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		raw      string
		canon    string
		language string
	}{
		// The ISO 639-2 duplicate pairs: one language, two published code
		// systems. Every pair must land on one canonical tag.
		"bibliographic german":   {"ger", "de", "de"},
		"terminological german":  {"deu", "de", "de"},
		"bibliographic czech":    {"cze", "cs", "cs"},
		"terminological czech":   {"ces", "cs", "cs"},
		"bibliographic chinese":  {"chi", "zh", "zh"},
		"terminological chinese": {"zho", "zh", "zh"},
		"bibliographic welsh":    {"wel", "cy", "cy"},
		"terminological welsh":   {"cym", "cy", "cy"},

		// Deprecated subtags still appear in old files.
		"deprecated indonesian": {"in", "id", "id"},
		"deprecated hebrew":     {"iw", "he", "he"},
		"deprecated yiddish":    {"ji", "yi", "yi"},

		// The reported bug: two spellings of Norwegian that must resolve to one
		// language even though they keep distinct canonical tags.
		"norwegian umbrella three letter": {"nor", "no", "no"},
		"norwegian bokmal three letter":   {"nob", "nb", "no"},
		"norwegian umbrella two letter":   {"no", "no", "no"},
		"norwegian bokmal two letter":     {"nb", "nb", "no"},
		"norwegian nynorsk":               {"nno", "nn", "nn"},

		// Macrolanguage members that fold onto the umbrella, because they are
		// the dominant variety.
		"mandarin":        {"cmn", "cmn", "zh"},
		"standard arabic": {"arb", "arb", "ar"},
		"coastal swahili": {"swh", "swh", "sw"},
		"filipino":        {"fil", "fil", "fil"},
		"tagalog":         {"tl", "fil", "fil"},

		// Macrolanguage members that do NOT fold, because they are not the
		// dominant variety.
		"cantonese":       {"yue", "yue", "yue"},
		"egyptian arabic": {"arz", "arz", "arz"},

		// Script and region subtags survive canonicalization.
		"simplified chinese":     {"zh-Hans", "zh-Hans", "zh"},
		"traditional chinese":    {"zh-Hant", "zh-Hant", "zh"},
		"brazilian portuguese":   {"pt-BR", "pt-BR", "pt"},
		"european portuguese":    {"pt-PT", "pt-PT", "pt"},
		"latin american spanish": {"es-419", "es-419", "es"},
		"serbian latin":          {"sr-Latn", "sr-Latn", "sr"},

		// Input hygiene: case and surrounding whitespace are not errors.
		"uppercase":        {"ENG", "en", "en"},
		"mixed case":       {"Pt-br", "pt-BR", "pt"},
		"leading space":    {"  fra", "fr", "fr"},
		"trailing newline": {"jpn\n", "ja", "ja"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := langtag.Parse(tc.raw)
			if !ok {
				t.Fatalf("Parse(%q) returned ok=false, want a valid tag", tc.raw)
			}
			if got.String() != tc.canon {
				t.Errorf("Parse(%q).String() = %q, want %q", tc.raw, got.String(), tc.canon)
			}
			if got.Language() != tc.language {
				t.Errorf("Parse(%q).Language() = %q, want %q", tc.raw, got.Language(), tc.language)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":                   "",
		"whitespace only":         "   ",
		"undetermined":            "und",
		"no linguistic content":   "zxx",
		"multiple languages":      "mul",
		"uncoded":                 "mis",
		"private use lower bound": "qaa",
		"private use upper bound": "qtz",
		"unknown three letter":    "pob",
		"not a tag":               "Norwegian Bokmål",
		"digits":                  "123",
		"punctuation":             "--",
		"overlong":                strings.Repeat("a", 64),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := langtag.Parse(raw)
			if ok {
				t.Errorf("Parse(%q) = (%q, true), want ok=false", raw, got.String())
			}
			if !got.IsZero() {
				t.Errorf("Parse(%q) returned non-zero Tag %q on failure, want the zero Tag", raw, got.String())
			}
		})
	}
}

// TestParseUndeterminedDoesNotInferEnglish pins that Parse uses Tag.Raw over
// Tag.Base: the underlying library answers "en" for the base of "und", which
// would turn every untagged track into English.
func TestParseUndeterminedDoesNotInferEnglish(t *testing.T) {
	t.Parallel()
	got, ok := langtag.Parse("und")
	if ok {
		t.Fatalf("Parse(%q) = (%q, true), want ok=false", "und", got.Language())
	}
	english := langtag.MustParse("en")
	if tier := langtag.Prefer(english).Compare(got); tier != langtag.TierNone {
		t.Errorf("Prefer(en).Compare(und) = %v, want %v", tier, langtag.TierNone)
	}
	if tier := langtag.Prefer(got).Compare(english); tier != langtag.TierNone {
		t.Errorf("Prefer(und).Compare(en) = %v, want %v", tier, langtag.TierNone)
	}
}

func TestZeroTagMatchesNothing(t *testing.T) {
	t.Parallel()
	var zero langtag.Tag
	if !zero.IsZero() {
		t.Error("langtag.Tag{}.IsZero() = false, want true")
	}
	if s := zero.String(); s != "" {
		t.Errorf("langtag.Tag{}.String() = %q, want %q", s, "")
	}
	// Two unknown tracks are not known to share a language, so even a zero Tag
	// against itself must not match.
	if tier := langtag.Prefer(zero).Compare(zero); tier != langtag.TierNone {
		t.Errorf("Prefer(zero).Compare(zero) = %v, want %v", tier, langtag.TierNone)
	}
	real := langtag.MustParse("en")
	if tier := langtag.Prefer(zero).Compare(real); tier != langtag.TierNone {
		t.Errorf("Prefer(zero).Compare(en) = %v, want %v", tier, langtag.TierNone)
	}
	if tier := langtag.Prefer(real).Compare(zero); tier != langtag.TierNone {
		t.Errorf("Prefer(en).Compare(zero) = %v, want %v", tier, langtag.TierNone)
	}
}

func TestValid(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"en": true, "nob": true, "pt-BR": true, "zh-Hant": true,
		"und": false, "zxx": false, "pob": false, "": false,
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got := langtag.Valid(raw); got != want {
				t.Errorf("Valid(%q) = %v, want %v", raw, got, want)
			}
		})
	}
}

func TestMustParsePanicsOnBadInput(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("MustParse(\"pob\") returned normally, want a panic")
		}
	}()
	_ = langtag.MustParse("pob")
}

// parseSeeds are FuzzParse's durable corpus, shared with the property tests.
// The non-ASCII seeds are one per class of Unicode 15 -> 17 change
// (documented in unicode_test.go).
var parseSeeds = []string{
	"", " ", "en", "ENG", "nob", "nor", "und", "zxx", "mul", "mis", "qaa",
	"pt-BR", "zh-Hant-HK", "es-419", "sr-Latn-RS", "x-private",
	"i-klingon", "art-lojban", "en-US-u-va-posix", "\x00", "\xff\xfe",
	"------", strings.Repeat("en-", 40), "en-" + strings.Repeat("a", 100),

	// Newly folding in Unicode 17, both members pre-existing: the only class
	// that changes strings.EqualFold for input a corpus can already hold.
	"\u0390n", "\u1FD3n", "\u03B0n", "\u1FE3n", "\uFB05k", "\uFB06k",
	// Category flips. U+0295 left Ll for Lo while already assigned, so
	// unicode.IsLower flips true -> false for real text; U+1171E left Mn
	// for Mc, the one change running the other way.
	"e\u0295", "en-\u1171Eatn",
	// Non-ASCII runes that a Unicode fold or lowercase maps onto ASCII, so
	// they impersonate a real subtag. Complete set for ASCII targets.
	"\u017Fk", "i\u017F", "\u212Ak", "\u0130d",
	// Gained a ToUpper mapping in Unicode 17 while already assigned.
	"e\u019B",
	// Newly assigned letter: unicode.IsLetter false on 15, true on 17.
	"\u105C0n",
}

// FuzzParse guards the untrusted-input boundary: language identifiers reach
// this package from Plex metadata, ffprobe output and third-party APIs.
func FuzzParse(f *testing.F) {
	for _, seed := range parseSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, ok := langtag.Parse(raw)
		if !ok {
			// A rejected tag must be the zero value, and must match nothing.
			if !got.IsZero() {
				t.Fatalf("Parse(%q) = (%q, false), want the zero Tag when ok is false", raw, got.String())
			}
			if tier := langtag.Prefer(got).Compare(got); tier != langtag.TierNone {
				t.Fatalf("Prefer(zero).Compare(zero) after Parse(%q) = %v, want %v", raw, tier, langtag.TierNone)
			}
			return
		}
		// An accepted tag is non-empty, names a language, and is a fixed point:
		// re-parsing its canonical form yields the same tag. Without this the
		// canonical form could not be used as a persistence key.
		if got.IsZero() {
			t.Fatalf("Parse(%q) = (zero, true), want a non-zero Tag when ok is true", raw)
		}
		if got.Language() == "" {
			t.Fatalf("Parse(%q).Language() = %q, want a language subtag", raw, "")
		}
		again, ok2 := langtag.Parse(got.String())
		if !ok2 {
			t.Fatalf("Parse(Parse(%q).String()) = ok=false, want ok=true (canonical form must re-parse)", raw)
		}
		if again != got {
			t.Fatalf("Parse(%q) is not idempotent: first %+v, second %+v", raw, got, again)
		}
		// A tag always matches itself exactly, at every floor.
		if tier := langtag.Prefer(got).Compare(got); tier != langtag.TierIdentical {
			t.Fatalf("Prefer(x).Compare(x) for Parse(%q) = %v, want %v", raw, tier, langtag.TierIdentical)
		}
		// Every accepted tag is ASCII, whatever arrived. BCP 47 subtags are
		// ASCII alphanumerics by grammar (RFC 5646 §2.1), and the canonical form
		// is a persistence key, so a non-ASCII byte reaching it would expose a
		// consumer's own comparison to the Unicode fold tables this package
		// deliberately never consults. Asserted here as a property so it holds
		// for generated input, not only for the seeds above.
		// A stated script is the resolved one: no table row overrides it.
		if stated := got.StatedScript(); stated != "" && got.Script() != stated {
			t.Fatalf("Parse(%q): Script() = %q, StatedScript() = %q, want them equal when a script is stated",
				raw, got.Script(), stated)
		}
		// MultiScript keys on the subtag or the macrolanguage, so a variety
		// of a multi-script macrolanguage is multi-script too.
		if base, ok := langtag.Parse(got.Language()); ok && base.MultiScript() && !got.MultiScript() {
			t.Fatalf("Parse(%q).MultiScript() = false, want true: its language %q is multi-script", raw, got.Language())
		}
		for _, out := range []string{got.String(), got.Language(), got.Script(), got.StatedScript(), got.Region()} {
			for i := range len(out) {
				if out[i] >= 0x80 {
					t.Fatalf("Parse(%+q) produced %+q, which carries a non-ASCII byte at %d", raw, out, i)
				}
			}
		}
	})
}

// parseCorpus adds to parseSeeds the stated-subtag shapes Parse canonicalizes:
// script only, region only, both, numeric regions and legacy forms that
// gain a script (sh) or a region (mo).
var parseCorpus = []string{
	"zh", "zh-Hans", "zh-Hant", "zh-TW", "zh-Hant-TW", "zh-419", "cmn-MY",
	"sr", "sr-Cyrl", "sr-ME", "sh", "mo", "ku-IQ", "ku-Latn-IQ", "kmr",
	"pt", "pt-PT", "pt-Latn-PT", "es-419", "en-001", "en-Zzzz", "en-ZZ",
	"nb", "no", "nn", "yue-Hans", "cnr", "cnr-ME", "ber",
}

// TestStatedSubtagsCanonical pins that StatedScript and Region read the
// canonical form, so they are "" exactly when String() states no such
// subtag, and that == stays canonical equality with the new fields.
func TestStatedSubtagsCanonical(t *testing.T) {
	t.Parallel()
	tags := make([]langtag.Tag, 0, len(parseCorpus)+len(parseSeeds))
	check := func(raw string, tag langtag.Tag) {
		tags = append(tags, tag)
		script, region := statedSubtags(tag.String())
		if got := tag.StatedScript(); got != script {
			t.Errorf("Parse(%q).StatedScript() = %q, want %q from %q", raw, got, script, tag.String())
		}
		if got := tag.Region(); got != region {
			t.Errorf("Parse(%q).Region() = %q, want %q from %q", raw, got, region, tag.String())
		}
	}
	for _, raw := range parseCorpus {
		tag, ok := langtag.Parse(raw)
		if !ok {
			t.Errorf("Parse(%q) rejected a corpus entry, want it accepted", raw)
			continue
		}
		check(raw, tag)
	}
	// Seeds include inputs Parse rejects; only the accepted ones are checked.
	for _, raw := range parseSeeds {
		if tag, ok := langtag.Parse(raw); ok {
			check(raw, tag)
		}
	}
	for _, a := range tags {
		for _, b := range tags {
			if (a == b) != (a.String() == b.String()) {
				t.Errorf("(%q == %q) = %v, want canonical equality", a, b, a == b)
			}
		}
	}
}

// statedSubtags splits a canonical lang[-Script][-Region] form.
func statedSubtags(canon string) (script, region string) {
	parts := strings.Split(canon, "-")[1:]
	if len(parts) > 0 && len(parts[0]) == 4 {
		script, parts = parts[0], parts[1:]
	}
	if len(parts) > 0 {
		region = parts[0]
	}
	return script, region
}
