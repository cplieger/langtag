package langtag_test

import (
	"testing"

	"github.com/cplieger/langtag/v3"
)

// TestResolvedScript pins the script an unstated-script tag resolves to
// against CLDR 48's likely subtags, the rows x/text v0.43.0's older snapshot
// answers differently (zh-MY Hant, ku-IQ Latn, ms-ID Arab, sd-IN Arab). Each
// of those was an automatic script crossing: a bare ku target took a ku-IQ
// file, which CLDR writes in Arabic, as the same script.
func TestResolvedScript(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		// Region rows decide where CLDR has one.
		"zh-TW":  "Hant",
		"hak-TW": "Hant",
		"ku-IQ":  "Arab",
		"ku-AM":  "Cyrl",
		"sd-IN":  "Deva",
		// No row for the region: the language row decides.
		"zh-MY":  "Hans",
		"zh-419": "Hans",
		"sr-RU":  "Cyrl",
		"ms-ID":  "Latn",
		// cmn has no row of its own; the macrolanguage zh answers.
		"cmn-MY": "Hans",
		// No rows of their own: the macrolanguage's region row decides.
		"cmn-TW": "Hant",
		"kmr-IQ": "Arab",
		"zsm-CC": "Arab",
		"uzn-AF": "Arab",
		// CLDR 48 holds no row for either, so the script is unknown.
		"cnr": "Zzzz",
		"ber": "Zzzz",
		// A stated script is never overridden by a row.
		"zh-Hant-MY": "Hant",
		"ku-Latn-IQ": "Latn",
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got := langtag.MustParse(raw).Script(); got != want {
				t.Errorf("MustParse(%q).Script() = %q, want %q", raw, got, want)
			}
		})
	}
}

// TestCloseScriptsPromoteOnlyWhatCLDRVouchesFor pins the one item moved
// between tiers: CLDR scores a generic same-language cross-script
// substitution at 50 and names only a handful of specific pairs, all but one
// a one-way transliteration row. The one symmetric pair is Serbian, so that
// is the whole population.
func TestCloseScriptsPromoteOnlyWhatCLDRVouchesFor(t *testing.T) {
	t.Parallel()
	t.Run("serbian latin and cyrillic", func(t *testing.T) {
		t.Parallel()
		a, b := langtag.MustParse("sr-Latn"), langtag.MustParse("sr-Cyrl")
		for _, pair := range [][2]langtag.Tag{{a, b}, {b, a}} {
			if got := langtag.Prefer(pair[0]).Compare(pair[1]); got != langtag.TierSameLanguage {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v; readers are taught both scripts",
					pair[0], pair[1], got, langtag.TierSameLanguage)
			}
		}
	})

	// Not promoted, each for a stated reason.
	notPromoted := map[string]struct {
		a, b string
		why  string
	}{
		"chinese simplified and traditional": {
			"zh-Hans", "zh-Hant",
			"CLDR names no distance for the pair, so it sits at the generic 50",
		},
		"uzbek scripts": {
			"uz-Latn", "uz-Cyrl",
			"upstream CLDR does not name it; only x/text's stale snapshot rated it close",
		},
		"azerbaijani scripts": {
			"az-Latn", "az-Cyrl",
			"same as Uzbek",
		},
		"japanese romanization": {
			"ja-Latn", "ja-Jpan",
			"one-way transliteration, not two audiences reading each other",
		},
		"hindi romanization": {
			"hi-Latn", "hi-Deva",
			"one-way transliteration",
		},
	}
	for name, tc := range notPromoted {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, b := langtag.MustParse(tc.a), langtag.MustParse(tc.b)
			if got := langtag.Prefer(a).Compare(b); got != langtag.TierOtherScript {
				t.Errorf("Prefer(%q).Compare(%q) = %v, want %v: %s",
					tc.a, tc.b, got, langtag.TierOtherScript, tc.why)
			}
		})
	}
}

// TestCloseScriptsStayWithinOneLanguage guards the obvious way a script
// promotion could go wrong: it must never bridge two languages. The table is
// emptied so that only the script rule could relate the pair.
func TestCloseScriptsStayWithinOneLanguage(t *testing.T) {
	t.Parallel()
	// Croatian and Bosnian are Serbian's nearest neighbours in Latin script, so
	// they are where a leak would show first.
	sr := langtag.WithFallbacks(nil).Prefer(langtag.MustParse("sr-Latn"))
	for _, other := range []string{"hr", "bs", "sl", "mk"} {
		if got := sr.Compare(langtag.MustParse(other)); got != langtag.TierNone {
			t.Errorf("Prefer(sr-Latn).Compare(%q) with no fallbacks = %v, want %v; a script promotion must not bridge languages",
				other, got, langtag.TierNone)
		}
	}
}

func TestCloseScriptsCarryProvenance(t *testing.T) {
	t.Parallel()
	list := langtag.CloseScripts()
	if len(list) == 0 {
		t.Fatal("CloseScripts() returned nothing")
	}
	for _, cs := range list {
		if cs.Language == "" || cs.A == "" || cs.B == "" {
			t.Errorf("entry %+v is incomplete", cs)
		}
		if cs.A == cs.B {
			t.Errorf("entry %+v names one script twice, so it cannot change any answer", cs)
		}
		// Tag.CoversScript drops its unknown-have test on this: a Zzzz row
		// would let an unknown script cover a known one.
		if cs.A == "Zzzz" || cs.B == "Zzzz" {
			t.Errorf("entry %+v pairs the unknown script Zzzz, which matches nothing", cs)
		}
		if cs.Reason == "" || cs.Provenance == "" {
			t.Errorf("entry %s %s/%s lacks a Reason or Provenance", cs.Language, cs.A, cs.B)
		}
		// The language must be spelled the way Tag.Language reports it, or the
		// entry silently never matches. Same trap the fallback table guards.
		tag, ok := langtag.Parse(cs.Language)
		if !ok || tag.Language() != cs.Language {
			t.Errorf("entry names language %q, which is not how Tag.Language spells it", cs.Language)
		}
	}
}

func TestCloseScriptsReturnsACopy(t *testing.T) {
	t.Parallel()
	first := langtag.CloseScripts()
	original := first[0]
	first[0] = langtag.CloseScript{Language: "xx", A: "Aaaa", B: "Bbbb"}
	if second := langtag.CloseScripts(); second[0] != original {
		t.Errorf("CloseScripts()[0] = %+v after a caller mutated an earlier result, want %+v",
			second[0], original)
	}
}
