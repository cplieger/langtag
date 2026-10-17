package names

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3"
)

func TestEnglish(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ tag, want string }{
		{"sr-Latn", "Serbian (Latin)"},
		{"pt-BR", "Portuguese (Brazil)"},
		{"zh-Hant", "Chinese (Traditional)"},
		{"zh-Hant-HK", "Chinese (Traditional, Hong Kong SAR China)"},
		{"nn", "Norwegian Nynorsk"},
		{"nb", "Norwegian Bokmål"},
		{"no", "Norwegian"},
		// CLDR 48 en does not name cnr; the ISO 639-2 file does.
		{"cnr", "Montenegrin"},
		{"zh", "Chinese"},
		// Only stated subtags are named: zh-TW resolves to Hant but states none.
		{"zh-TW", "Chinese (Taiwan)"},
		{"es-419", "Spanish (Latin America)"},
		// A subtag CLDR en does not name is written as its code.
		{"es-AA", "Spanish (AA)"},
		{"sr-Qaaa", "Serbian (Qaaa)"},
		{"sr-Qaaa-AA", "Serbian (Qaaa, AA)"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			got, ok := English(langtag.MustParse(tc.tag))
			if !ok || got != tc.want {
				t.Errorf("English(%s) = %q, %v; want %q, true", tc.tag, got, ok, tc.want)
			}
		})
	}
}

func TestComposeFillsOnce(t *testing.T) {
	t.Parallel()
	if got, want := compose(localePattern, "{1}", "x"), "{1} (x)"; got != want {
		t.Errorf("compose with a placeholder in a name = %q, want %q", got, want)
	}
}

func TestEnglishUnnamed(t *testing.T) {
	t.Parallel()
	for name, tag := range map[string]langtag.Tag{
		"zero": {},
		// cmn parses and keeps its own subtag, which neither source names.
		"cmn": langtag.MustParse("cmn"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, ok := English(tag); ok || got != "" {
				t.Errorf("English(%q) = %q, %v; want \"\", false", tag, got, ok)
			}
		})
	}
}

func TestLanguagesReturnsCopy(t *testing.T) {
	t.Parallel()
	first := Languages()
	want := slices.Clone(first)
	for i := range first {
		first[i] = Language{"xx", "Mutated"}
	}
	if got := Languages(); !slices.Equal(got, want) {
		t.Errorf("Languages after mutating an earlier result differs from the first result")
	}
}

func TestLanguages(t *testing.T) {
	t.Parallel()
	got := Languages()
	byCode := func(l Language, code string) int { return strings.Compare(l.Code, code) }
	for _, want := range []Language{
		{"nb", "Norwegian Bokmål"},
		{"nn", "Norwegian Nynorsk"},
		{"no", "Norwegian"},
		{"cnr", "Montenegrin"},
		{"fil", "Filipino"},
		{"zh", "Chinese"},
		// Codes Parse canonicalizes to another keep their own names.
		{"tl", "Tagalog"},
		{"sh", "Serbo-Croatian"},
	} {
		i, found := slices.BinarySearchFunc(got, want.Code, byCode)
		if !found || got[i] != want {
			t.Errorf("Languages lacks %v", want)
		}
	}
	if !slices.IsSortedFunc(got, func(a, b Language) int { return strings.Compare(a.Code, b.Code) }) {
		t.Error("Languages is not sorted by code")
	}
}

// TestEveryLanguageReachable pins that English names every language the
// table lists that Parse accepts: a row keyed by a code Parse canonicalizes
// away is reachable only if the canonical subtag has a row too.
func TestEveryLanguageReachable(t *testing.T) {
	t.Parallel()
	for _, l := range Languages() {
		tag, ok := langtag.Parse(l.Code)
		if !ok {
			continue
		}
		if _, named := English(tag); !named {
			t.Errorf("English(Parse(%q)) = false; %s (%s) is listed but unreachable", l.Code, tag, l.Name)
		}
	}
}

// TestTablesSorted pins lookup's precondition: a table out of order or
// holding a code twice makes the binary search miss names it holds.
func TestTablesSorted(t *testing.T) {
	t.Parallel()
	for name, table := range map[string][]entry{"languages": languages[:], "scripts": scripts[:], "regions": regions[:]} {
		for i := 1; i < len(table); i++ {
			if table[i-1].code >= table[i].code {
				t.Errorf("%s: %q precedes %q, want strictly increasing codes", name, table[i-1].code, table[i].code)
			}
		}
	}
}

// TestNoInitWork pins that importing the package costs no init work: an
// import-only program run with GODEBUG=inittrace=1 reports no init for it.
// The program's own map literal is the control proving the trace is on.
func TestNoInitWork(t *testing.T) {
	t.Parallel()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	goVersion := goDirective(t, filepath.Join(root, "go.mod"))
	dir := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":  "module initprobe\n\ngo " + goVersion + "\n",
		"go.work": "go " + goVersion + "\n\nuse (\n\t.\n\t" + strconv.Quote(root) + "\n)\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t_ \"" + namesPath + "\"\n)\n\n" +
			"var control = map[string]int{\"x\": 1}\n\nfunc main() { fmt.Println(len(control)) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "initprobe")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.CommandContext(t.Context(), goBin, "build", "-o", bin, ".")
	build.Dir = dir
	// An ambient -mod=mod is refused in workspace mode.
	build.Env = append(os.Environ(), "GOWORK="+filepath.Join(dir, "go.work"), "GOFLAGS=")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build of the import-only program: %v\n%s", err, out)
	}
	probe := exec.CommandContext(t.Context(), bin)
	probe.Env = append(os.Environ(), "GODEBUG=inittrace=1")
	out, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("running the import-only program: %v\n%s", err, out)
	}
	trace := string(out)
	if !strings.Contains(trace, "init main @") {
		t.Fatalf("no inittrace line for the program's own map, so the trace is off:\n%s", trace)
	}
	if strings.Contains(trace, "init "+namesPath+" @") {
		t.Errorf("importing %s does init work, want none:\n%s", namesPath, trace)
	}
}

const namesPath = "github.com/cplieger/langtag/v3/names"

// goDirective returns the go line's version from the go.mod at path.
func goDirective(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			return v
		}
	}
	t.Fatalf("%s has no go line", path)
	return ""
}

const namesGolden = "testdata/names.golden"

// composedCorpus holds the composed names the golden records beside the
// table rows.
var composedCorpus = []string{
	"cnr", "es-419", "es-AA", "es-ES", "fil", "fr-CA", "fr-FR", "nb", "nn", "no",
	"pt", "pt-BR", "pt-PT", "sr-Cyrl", "sr-Latn", "sr-Latn-RS", "sr-Qaaa",
	"yue", "zh", "zh-Hans", "zh-Hant", "zh-Hant-HK", "zh-TW",
}

func goldenText(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("# langtag English names, generated from CLDR en and ISO 639-2.\n")
	b.WriteString("# Regenerate with UPDATE_GOLDEN=1 go test -run TestNamesGolden ./names\n")
	for _, table := range []struct {
		kind string
		rows []entry
	}{{"language", languages[:]}, {"script", scripts[:]}, {"region", regions[:]}} {
		for _, e := range table.rows {
			b.WriteString(table.kind + " " + e.code + " " + e.name + "\n")
		}
	}
	for _, tag := range composedCorpus {
		name, ok := English(langtag.MustParse(tag))
		if !ok {
			t.Fatalf("English(%s) names nothing", tag)
		}
		b.WriteString("composed " + tag + " " + name + "\n")
	}
	return b.Bytes()
}

// TestNamesGolden makes every name change a reviewed diff: a regeneration
// or a hand edit of table.go that changes a row or a composed name fails
// here until the same change updates the golden.
func TestNamesGolden(t *testing.T) {
	want := goldenText(t)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(filepath.FromSlash(namesGolden), want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(filepath.FromSlash(namesGolden))
	if err != nil {
		t.Fatalf("read %s: %v; regenerate with UPDATE_GOLDEN=1 go test -run TestNamesGolden ./names", namesGolden, err)
	}
	if bytes.Equal(got, want) {
		return
	}
	gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		g, w := lineAt(gotLines, i), lineAt(wantLines, i)
		if g != w {
			t.Errorf("%s line %d is %q, the table gives %q; review the change, then regenerate with UPDATE_GOLDEN=1 go test -run TestNamesGolden ./names", namesGolden, i+1, g, w)
			return
		}
	}
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end of file>"
}
