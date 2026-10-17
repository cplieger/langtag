package langtag

import (
	"bytes"
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

var (
	releaseShape = regexp.MustCompile(`\brelease-\d+(?:-\d+)?\b`)
	versionShape = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)
	sumShape     = regexp.MustCompile(`\b[0-9a-f]{64}\b`)
)

// TestOnePin checks every generated table's header names the release and
// the SHA-256 sums internal/gen/cldr pins its inputs to, so a table left
// behind by a pin move, or carrying a pin of its own, fails here.
func TestOnePin(t *testing.T) {
	t.Parallel()
	for _, table := range []struct {
		path, tagLine string
		files         []cldr.File
	}{
		{"scripttable.go", "// CLDR tag: " + cldr.Release + "\n", []cldr.File{cldr.LikelySubtags}},
		{"regiontable.go", "// CLDR tag: " + cldr.Release + "\n", []cldr.File{cldr.SupplementalData, cldr.LanguageInfo}},
		{"names/table.go", "// cldr-json tag: " + cldr.JSONVersion + "\n", []cldr.File{cldr.Languages, cldr.Scripts, cldr.Territories, cldr.LocaleDisplayNames, cldr.ISO6392}},
	} {
		t.Run(table.path, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(table.path)
			if err != nil {
				t.Fatal(err)
			}
			header, _, found := bytes.Cut(b, []byte("\npackage "))
			if !found {
				t.Fatalf("%s has no package clause", table.path)
			}
			if !bytes.Contains(header, []byte(table.tagLine)) {
				t.Errorf("%s lacks the header line %q: run go generate ./...", table.path, table.tagLine)
			}
			for _, m := range releaseShape.FindAll(header, -1) {
				if string(m) != cldr.Release {
					t.Errorf("%s names CLDR %s, want the pinned %s: run go generate ./...", table.path, m, cldr.Release)
				}
			}
			for _, m := range versionShape.FindAll(header, -1) {
				if string(m) != cldr.JSONVersion {
					t.Errorf("%s names cldr-json %s, want the pinned %s: run go generate ./...", table.path, m, cldr.JSONVersion)
				}
			}
			want := map[string]bool{}
			for _, f := range table.files {
				want[f.SHA256()] = true
			}
			got := map[string]bool{}
			for _, m := range sumShape.FindAll(header, -1) {
				got[string(m)] = true
			}
			if !maps.Equal(got, want) {
				t.Errorf("%s names the SHA-256 sums %v, want its inputs' pinned %v: run go generate ./...",
					table.path, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
			}
		})
	}
}
