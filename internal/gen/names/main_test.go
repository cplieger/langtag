package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

// fixture describes one set of generator inputs.
type fixture struct {
	languages, scripts, territories map[string]string
	pattern, separator              string
	locale                          string
	iso                             string
}

func baseFixture() fixture {
	return fixture{
		languages: map[string]string{
			"bn": "Bangla", "en": "English", "pt": "Portuguese", "zh": "Chinese",
			// tl canonicalizes to fil, which has its own row.
			"fil": "Filipino", "tl": "Tagalog",
			// A combined entry and an alternative form, both dropped.
			"pt-BR": "Brazilian Portuguese", "zh-alt-long": "Mandarin Chinese",
		},
		scripts:     map[string]string{"Hant": "Traditional", "Latn": "Latin", "Hant-alt-stand-alone": "Traditional Han"},
		territories: map[string]string{"BR": "Brazil", "419": "Latin America", "HK": "Hong Kong SAR China", "HK-alt-short": "Hong Kong"},
		pattern:     "{0} ({1})",
		separator:   "{0}, {1}",
		locale:      "en",
		iso: "\uFEFFben||bn|Bengali|bengali\n" +
			"bih|||Bihari languages|langues biharis\n" +
			"cat||ca|Catalan; Valencian|catalan; valencien\n" +
			"cnr|||Montenegrin|monténégrin\n" +
			"fre|fra|fr|French|français\n" +
			"qaa-qtz|||Reserved for local use|réservée à l'usage local\n" +
			"tib|bod|bo|Tibetan|tibétain\n" +
			"xxb|xxt||Fixture|fixture\n",
	}
}

func cldrJSON(t *testing.T, locale, field string, value any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"main": map[string]any{locale: map[string]any{
		"identity":           map[string]string{"language": locale},
		"localeDisplayNames": map[string]any{field: value},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (f fixture) inputs(t *testing.T) inputs {
	t.Helper()
	var in inputs
	in.cldr[languagesFile] = cldrJSON(t, f.locale, "languages", f.languages)
	in.cldr[scriptsFile] = cldrJSON(t, f.locale, "scripts", f.scripts)
	in.cldr[territoriesFile] = cldrJSON(t, f.locale, "territories", f.territories)
	in.cldr[displayFile] = cldrJSON(t, f.locale, "localeDisplayPattern", map[string]string{"localePattern": f.pattern, "localeSeparator": f.separator})
	in.iso = []byte(f.iso)
	return in
}

// serve returns a readFunc answering each file of in.
func serve(in *inputs) readFunc {
	return func(_ context.Context, f cldr.File) ([]byte, error) {
		if f == cldr.ISO6392 {
			return in.iso, nil
		}
		for i, c := range cldrFiles {
			if f == c {
				return in.cldr[i], nil
			}
		}
		return nil, fmt.Errorf("no fixture for %s", f.Name())
	}
}

// moduleRoot returns a directory holding an empty names/.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "names"), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRunWritesTheTable checks run writes exactly generate's table for the
// pinned files, its header naming the pinned cldr-json release and each
// file's SHA-256.
func TestRunWritesTheTable(t *testing.T) {
	t.Parallel()
	root, in := moduleRoot(t), baseFixture().inputs(t)
	if err := run(t.Context(), root, serve(&in)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tableFile)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := generate(&in, cldr.JSONVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("run wrote:\n%s\nwant:\n%s", got, want)
	}
	header := []string{
		"// cldr-json tag: " + cldr.JSONVersion + "\n",
		"// Source: https://raw.githubusercontent.com/unicode-org/cldr-json/" + cldr.JSONVersion + "/cldr-json/cldr-localenames-full/main/en/*.json\n",
		"// ISO 639-2 SHA-256: " + cldr.SHA256(in.iso) + "\n",
	}
	for i, f := range cldrFiles {
		header = append(header, "// "+f.Base()+" SHA-256: "+cldr.SHA256(in.cldr[i])+"\n")
	}
	for _, line := range header {
		if !bytes.Contains(got, []byte(line)) {
			t.Errorf("the table header lacks %q", line)
		}
	}
}

// TestRunRefusalWritesNothing checks a read the pin refuses, the ISO 639-2
// file's included, leaves no table.
func TestRunRefusalWritesNothing(t *testing.T) {
	t.Parallel()
	for _, refused := range append(cldrFiles[:], cldr.ISO6392) {
		t.Run(refused.Base(), func(t *testing.T) {
			t.Parallel()
			root, in := moduleRoot(t), baseFixture().inputs(t)
			errPin := errors.New(refused.Base() + " has SHA-256 0, want the pinned 1")
			read := func(ctx context.Context, f cldr.File) ([]byte, error) {
				if f == refused {
					return nil, errPin
				}
				return serve(&in)(ctx, f)
			}
			if err := run(t.Context(), root, read); !errors.Is(err, errPin) {
				t.Errorf("run over a refused %s = %v, want %v", refused.Base(), err, errPin)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(tableFile))); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("run left %s after a refused read (stat: %v)", tableFile, err)
			}
		})
	}
}

func TestGenerateRows(t *testing.T) {
	t.Parallel()
	in := baseFixture().inputs(t)
	table, err := generate(&in, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"bn", "Bangla"}`,       // CLDR wins over ISO 639-2's "Bengali"
		`{"cnr", "Montenegrin"}`, // ISO 639-2 fills a language CLDR leaves unnamed
		`{"ca", "Catalan"}`,      // the first of several ISO names
		`{"fr", "French"}`,       // keyed by the two-letter code
		`{"bo", "Tibetan"}`,
		`{"xxt", "Fixture"}`, // the terminology code before the bibliographic one
		`{"bih", "Bihari languages"}`,
		`{"bh", "Bihari languages"}`, // under the subtag Parse canonicalizes bih to
		`{"tl", "Tagalog"}`,
		`{"fil", "Filipino"}`,
		`{"Hant", "Traditional"}`,
		`{"HK", "Hong Kong SAR China"}`,
		`{"419", "Latin America"}`,
		`localePattern   = "{0} ({1})"`,
		`localeSeparator = "{0}, {1}"`,
	} {
		if !bytes.Contains(table, []byte(want)) {
			t.Errorf("the table lacks %s", want)
		}
	}
	for _, absent := range []string{"Brazilian Portuguese", "Mandarin Chinese", "Traditional Han", `"Hong Kong"`, "Bengali", "Valencian", "qaa", "xxb", "fre", "bod", `{"fil", "Tagalog"}`} {
		if bytes.Contains(table, []byte(absent)) {
			t.Errorf("the table holds %s", absent)
		}
	}
}

// TestGenerateAgreeingCodesShareTheirName checks codes that canonicalize to
// one unnamed subtag under the same name give it that name.
func TestGenerateAgreeingCodesShareTheirName(t *testing.T) {
	t.Parallel()
	f := baseFixture()
	f.languages["iw"], f.languages["heb"] = "Hebrew", "Hebrew"
	in := f.inputs(t)
	table, err := generate(&in, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(table, []byte(`{"he", "Hebrew"}`)) {
		t.Errorf("the table lacks he Hebrew:\n%s", table)
	}
}

// TestParseISOReadsCRLF pins the file as the Registration Authority serves
// it: a byte order mark and CRLF line ends.
func TestParseISOReadsCRLF(t *testing.T) {
	t.Parallel()
	got, err := parseISO([]byte("\uFEFFaar||aa|Afar|afar\r\nabk||ab|Abkhazian|abkhaze\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["aa"] != "Afar" || got["ab"] != "Abkhazian" {
		t.Errorf("parseISO over a BOM and CRLF lines = %v, want aa Afar and ab Abkhazian", got)
	}
}

func TestGenerateRefusesMalformed(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(*fixture){
		"bad language":      func(f *fixture) { f.languages["EN"] = "English" },
		"bad script":        func(f *fixture) { f.scripts["latn"] = "Latin" },
		"bad territory":     func(f *fixture) { f.territories["B"] = "Brazil" },
		"empty name":        func(f *fixture) { f.languages["de"] = "" },
		"untrimmed name":    func(f *fixture) { f.languages["de"] = "German " },
		"control in name":   func(f *fixture) { f.territories["DE"] = "Germ\x00any" },
		"format in name":    func(f *fixture) { f.languages["de"] = "\u202eGerman" },
		"control pattern":   func(f *fixture) { f.pattern = "{0}\n({1})" },
		"control separator": func(f *fixture) { f.separator = "{0},\x07{1}" },
		"iso format name":   func(f *fixture) { f.iso = "ger|deu|de|Ger\u200bman|allemand\n" },
		"no languages":      func(f *fixture) { f.languages = map[string]string{"pt-BR": "Brazilian Portuguese"} },
		"pattern":           func(f *fixture) { f.pattern = "{0}" },
		"separator":         func(f *fixture) { f.separator = "{0}, {1}, {1}" },
		"other locale":      func(f *fixture) { f.locale = "de" },
		"iso fields":        func(f *fixture) { f.iso = "ger|deu|de|German\n" },
		"iso code":          func(f *fixture) { f.iso = "GER|deu|de|German|allemand\n" },
		"iso alpha2":        func(f *fixture) { f.iso = "ger|deu|deu|German|allemand\n" },
		"iso repeat":        func(f *fixture) { f.iso = "ger|deu|de|German|allemand\nger|deu|de|German|allemand\n" },
		"iso empty":         func(f *fixture) { f.iso = "\n" },
		"iso empty name":    func(f *fixture) { f.iso = "ger|deu|de||allemand\n" },
		"iso only reserve":  func(f *fixture) { f.iso = "qaa-qtz|||Reserved for local use|réservée\n" },
		// iw and heb both canonicalize to he, which has no row.
		"canonical clash": func(f *fixture) { f.languages["iw"], f.languages["heb"] = "Hebrew", "Ivrit" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := baseFixture()
			edit(&f)
			in := f.inputs(t)
			if _, err := generate(&in, "fixture"); err == nil {
				t.Errorf("generate over the %s fixture succeeded, want a refusal", name)
			}
		})
	}
	for name, src := range map[string]string{
		"not json":       `{"main":`,
		"other identity": `{"main":{"en":{"identity":{"language":"fr"},"localeDisplayNames":{"scripts":{"Latn":"Latin"}}}}}`,
		"invalid utf-8":  "{\"main\":{\"en\":{\"identity\":{\"language\":\"en\"},\"localeDisplayNames\":{\"scripts\":{\"Latn\":\"Lat\xffin\"}}}}}",
		"repeated name":  `{"main":{"en":{"identity":{"language":"en"},"localeDisplayNames":{"scripts":{"Latn":"Latin","Latn":"Other"}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := baseFixture().inputs(t)
			in.cldr[scriptsFile] = []byte(src)
			if _, err := generate(&in, "fixture"); err == nil {
				t.Errorf("generate over %s succeeded, want a refusal", src)
			}
		})
	}
}
