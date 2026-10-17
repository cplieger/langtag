// Command names writes the English name table of langtag/names,
// names/table.go, from CLDR en's display names in cldr-json and, for a
// language CLDR en leaves unnamed, the ISO 639-2 Registration Authority's
// English name, each read at the pin internal/gen/cldr holds.
//
//	go run ./internal/gen/names    regenerate the table
//
// Run from the module root, or pass -root; go generate runs it from names/.
package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cplieger/langtag/v3"
	"github.com/cplieger/langtag/v3/internal/gen/cldr"
	"github.com/cplieger/langtag/v3/internal/gen/subtag"
)

const tableFile = "names/table.go"

// cldrFiles are the cldr-json en files, indexing inputs.cldr.
var cldrFiles = [...]cldr.File{cldr.Languages, cldr.Scripts, cldr.Territories, cldr.LocaleDisplayNames}

const (
	languagesFile = iota
	scriptsFile
	territoriesFile
	displayFile
)

// inputs are the read files, cldr indexed like cldrFiles.
type inputs struct {
	cldr [len(cldrFiles)][]byte
	iso  []byte
}

func main() {
	root := flag.String("root", ".", "the module root")
	flag.Parse()
	if err := run(context.Background(), *root, cldr.Read); err != nil {
		fmt.Fprintln(os.Stderr, "names:", err)
		os.Exit(1)
	}
}

// readFunc returns a file at the pin, refusing other bytes; cldr.Read
// outside tests.
type readFunc func(ctx context.Context, f cldr.File) ([]byte, error)

// run writes root's table from the pinned files.
func run(ctx context.Context, root string, read readFunc) error {
	var in inputs
	for i, f := range cldrFiles {
		b, err := read(ctx, f)
		if err != nil {
			return err
		}
		in.cldr[i] = b
	}
	iso, err := read(ctx, cldr.ISO6392)
	if err != nil {
		return err
	}
	in.iso = iso
	table, err := generate(&in, cldr.JSONVersion)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(tableFile)), table, 0o600)
}

// generate returns table.go's source for in, read at cldr-json release
// jsonVersion, naming each file's SHA-256 in its header.
func generate(in *inputs, jsonVersion string) ([]byte, error) {
	t, err := readTables(in)
	if err != nil {
		return nil, err
	}
	return render(in, jsonVersion, &t)
}

// tables are the rows and patterns table.go holds.
type tables struct {
	languages, scripts, regions map[string]string
	pattern, separator          string
}

// readTables parses in, filling the languages CLDR leaves unnamed from the
// ISO 639-2 file.
func readTables(in *inputs) (tables, error) {
	var names [len(cldrFiles)]displayNames
	for i, f := range cldrFiles {
		n, err := parseCLDR(f.Base(), in.cldr[i])
		if err != nil {
			return tables{}, err
		}
		names[i] = n
	}
	var t tables
	var err error
	if t.languages, err = table(names[languagesFile].Languages, subtag.Language, "language"); err != nil {
		return tables{}, err
	}
	if t.scripts, err = table(names[scriptsFile].Scripts, subtag.Script, "script"); err != nil {
		return tables{}, err
	}
	if t.regions, err = table(names[territoriesFile].Territories, subtag.Region, "territory"); err != nil {
		return tables{}, err
	}
	iso, err := parseISO(in.iso)
	if err != nil {
		return tables{}, err
	}
	for code, name := range iso {
		if _, named := t.languages[code]; !named {
			t.languages[code] = name
		}
	}
	if err := addCanonical(t.languages); err != nil {
		return tables{}, err
	}
	t.pattern = names[displayFile].LocaleDisplayPattern.LocalePattern
	t.separator = names[displayFile].LocaleDisplayPattern.LocaleSeparator
	for _, pat := range []string{t.pattern, t.separator} {
		if !printable(pat) || strings.Count(pat, "{0}") != 1 || strings.Count(pat, "{1}") != 1 {
			return tables{}, fmt.Errorf("localeDisplayPattern %q: want printable UTF-8 with {0} and {1} once each", pat)
		}
	}
	return t, nil
}

// addCanonical adds each language's name under the subtag Parse
// canonicalizes its code to, where that subtag has no row: names.English
// looks a tag up by its canonical subtag, so the row would be unreachable.
// It refuses two codes giving one such subtag different names.
func addCanonical(rows map[string]string) error {
	from := map[string]string{}
	for _, code := range slices.Sorted(maps.Keys(rows)) {
		tag, ok := langtag.Parse(code)
		if !ok {
			continue
		}
		canon, _, _ := strings.Cut(tag.String(), "-")
		if _, named := rows[canon]; named {
			continue
		}
		if other, dup := from[canon]; dup && rows[other] != rows[code] {
			return fmt.Errorf("languages %s (%q) and %s (%q) both canonicalize to %s", other, rows[other], code, rows[code], canon)
		}
		from[canon] = code
	}
	for canon, code := range from {
		rows[canon] = rows[code]
	}
	return nil
}

// render returns table.go's source: a header naming every input of in, read
// at cldr-json release jsonVersion, and its SHA-256, then the two patterns
// and the three tables sorted by code.
func render(in *inputs, jsonVersion string, t *tables) ([]byte, error) {
	pin := cldr.Pin{JSONVersion: jsonVersion}
	var b bytes.Buffer
	b.WriteString("// Code generated by go run ./internal/gen/names; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "// Source: %s*.json\n", strings.TrimSuffix(cldrFiles[0].URL(pin), cldrFiles[0].Base()))
	fmt.Fprintf(&b, "// cldr-json tag: %s\n", jsonVersion)
	for i, f := range cldrFiles {
		fmt.Fprintf(&b, "// %s SHA-256: %s\n", f.Base(), cldr.SHA256(in.cldr[i]))
	}
	fmt.Fprintf(&b, "// Fallback: %s\n// ISO 639-2 SHA-256: %s\n\npackage names\n\n", cldr.ISO6392.URL(pin), cldr.SHA256(in.iso))
	fmt.Fprintf(&b, "const (\n\tlocalePattern   = %q\n\tlocaleSeparator = %q\n)\n\n", t.pattern, t.separator)
	writeTable(&b, "languages", t.languages)
	writeTable(&b, "scripts", t.scripts)
	writeTable(&b, "regions", t.regions)
	return format.Source(b.Bytes())
}

func writeTable(b *bytes.Buffer, name string, rows map[string]string) {
	fmt.Fprintf(b, "var %s = [...]entry{\n", name)
	for _, code := range slices.Sorted(maps.Keys(rows)) {
		fmt.Fprintf(b, "\t{%q, %q},\n", code, rows[code])
	}
	b.WriteString("}\n\n")
}

// displayNames is the part of a cldr-json en localenames file the table
// reads; each file fills one field.
type displayNames struct {
	Languages            map[string]string `json:"languages"`
	Scripts              map[string]string `json:"scripts"`
	Territories          map[string]string `json:"territories"`
	LocaleDisplayPattern struct {
		LocalePattern   string `json:"localePattern"`
		LocaleSeparator string `json:"localeSeparator"`
	} `json:"localeDisplayPattern"`
}

// parseCLDR returns the display names of src, a cldr-json en file, refusing a
// file without the en locale. encoding/json/v2, unlike v1, refuses invalid
// UTF-8 and a repeated object name rather than repairing or overwriting.
func parseCLDR(file string, src []byte) (displayNames, error) {
	var doc struct {
		Main map[string]struct {
			Identity struct {
				Language string `json:"language"`
			} `json:"identity"`
			LocaleDisplayNames displayNames `json:"localeDisplayNames"`
		} `json:"main"`
	}
	if err := json.Unmarshal(src, &doc); err != nil {
		return displayNames{}, fmt.Errorf("parse %s: %w", file, err)
	}
	en, ok := doc.Main["en"]
	if !ok || en.Identity.Language != "en" {
		return displayNames{}, fmt.Errorf("%s: want the en locale", file)
	}
	return en.LocaleDisplayNames, nil
}

// table returns the rows of src keyed by a plain subtag. A key holding "-"
// is a combined locale ("pt-BR") or an alternative form ("HK-alt-short"),
// which names never use; every other key must match shape.
func table(src map[string]string, shape *regexp.Regexp, what string) (map[string]string, error) {
	out := make(map[string]string, len(src))
	for code, name := range src {
		if strings.Contains(code, "-") {
			continue
		}
		if !shape.MatchString(code) {
			return nil, fmt.Errorf("%s %q: malformed subtag", what, code)
		}
		if err := checkName(name); err != nil {
			return nil, fmt.Errorf("%s %s: %w", what, code, err)
		}
		out[code] = name
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no %s names", what)
	}
	return out, nil
}

// checkName refuses a name a picker could not show as it stands.
func checkName(name string) error {
	if name == "" || strings.TrimSpace(name) != name || !printable(name) {
		return fmt.Errorf("name %q: want trimmed, printable UTF-8", name)
	}
	return nil
}

// printable reports whether s is valid UTF-8 whose every rune passes
// unicode.IsPrint: U+0020 is the one space it admits, and it refuses C0 and
// C1 controls and format characters such as U+202E.
func printable(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) })
}

var (
	isoAlpha3 = regexp.MustCompile(`^[a-z]{3}$`)
	isoAlpha2 = regexp.MustCompile(`^[a-z]{2}$`)
)

// isoReserved is the ISO 639-2 row naming the private-use range, which
// names no language.
const isoReserved = "qaa-qtz"

// parseISO returns the first English name of each ISO 639-2 language,
// keyed by its two-letter code, else its terminology code, else its
// bibliographic code, the order RFC 5646 section 2.2.1 registers them in.
// src is the Registration Authority's UTF-8 file, one
// "bib|term|alpha2|English|French" row per line.
func parseISO(src []byte) (map[string]string, error) {
	src = bytes.TrimPrefix(src, []byte("\uFEFF"))
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(src))
	for line := 1; sc.Scan(); line++ {
		if sc.Text() == "" {
			continue
		}
		code, name, err := parseISORow(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("ISO 639-2 line %d: %w", line, err)
		}
		if code == "" {
			continue
		}
		if _, dup := out[code]; dup {
			return nil, fmt.Errorf("ISO 639-2 line %d repeats the code %s", line, code)
		}
		out[code] = name
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read ISO 639-2: %w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("ISO 639-2 file holds no languages")
	}
	return out, nil
}

// parseISORow returns one row's code and first English name, or an empty
// code for the private-use range.
func parseISORow(text string) (code, name string, err error) {
	f := strings.Split(text, "|")
	if len(f) != 5 {
		return "", "", fmt.Errorf("want 5 fields, got %d", len(f))
	}
	bib, term, alpha2, english := f[0], f[1], f[2], f[3]
	if bib == isoReserved {
		return "", "", nil
	}
	if !isoAlpha3.MatchString(bib) || (term != "" && !isoAlpha3.MatchString(term)) || (alpha2 != "" && !isoAlpha2.MatchString(alpha2)) {
		return "", "", fmt.Errorf("malformed code in %q", text)
	}
	name, _, _ = strings.Cut(english, "; ")
	if err := checkName(name); err != nil {
		return "", "", err
	}
	return cmp.Or(alpha2, term, bib), name, nil
}
