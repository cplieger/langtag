package langtag_test

import (
	"strings"
	"testing"

	"github.com/cplieger/langtag/v2"
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

// TestParseUndeterminedDoesNotInferEnglish pins the trap that motivated using
// Tag.Raw over Tag.Base during parsing: the underlying library answers "en" for
// the base of "und", so a naive implementation turns every untagged track into
// English.
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

// FuzzParse guards the untrusted-input boundary. Language identifiers reach
// this package from Plex metadata, ffprobe output and third-party APIs, so
// every invariant a caller relies on is asserted here rather than assumed.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"", " ", "en", "ENG", "nob", "nor", "und", "zxx", "mul", "mis", "qaa",
		"pt-BR", "zh-Hant-HK", "es-419", "sr-Latn-RS", "x-private",
		"i-klingon", "art-lojban", "en-US-u-va-posix", "\x00", "\xff\xfe",
		"------", strings.Repeat("en-", 40), "en-" + strings.Repeat("a", 100),
	} {
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
	})
}
