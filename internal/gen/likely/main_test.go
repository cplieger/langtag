package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// serve returns a readFunc answering likelySubtags.xml with src.
func serve(t *testing.T, src []byte) readFunc {
	t.Helper()
	return func(_ context.Context, f cldr.File) ([]byte, error) {
		if f != cldr.LikelySubtags {
			t.Errorf("read %s, want only likelySubtags.xml", f.Name())
		}
		return src, nil
	}
}

// TestRunWritesTheTable checks run writes exactly generate's table for the
// pinned bytes, its header naming the pinned release and the bytes' SHA-256.
func TestRunWritesTheTable(t *testing.T) {
	t.Parallel()
	dir, src := t.TempDir(), fixture(t, "likely-old.xml")
	if err := run(t.Context(), dir, serve(t, src)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, tableFile))
	if err != nil {
		t.Fatal(err)
	}
	want, err := generate(src, cldr.Release)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("run wrote:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range []string{"// CLDR tag: " + cldr.Release + "\n", "// SHA-256: " + cldr.SHA256(src) + "\n", "/cldr/" + cldr.Release + "/common/supplemental/likelySubtags.xml\n"} {
		if !bytes.Contains(got, []byte(line)) {
			t.Errorf("the table header lacks %q", line)
		}
	}
}

// TestRunRefusalWritesNothing checks a read the pin refuses leaves no table.
func TestRunRefusalWritesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	errPin := errors.New("likelySubtags.xml has SHA-256 0, want the pinned 1")
	read := func(context.Context, cldr.File) ([]byte, error) { return nil, errPin }
	if err := run(t.Context(), dir, read); !errors.Is(err, errPin) {
		t.Errorf("run over a refused read = %v, want %v", err, errPin)
	}
	if _, err := os.Stat(filepath.Join(dir, tableFile)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("run left %s after a refused read (stat: %v)", tableFile, err)
	}
}

// TestRunReportsAnUnwritableTable checks a table that cannot be written
// fails the run instead of leaving the old table looking regenerated.
func TestRunReportsAnUnwritableTable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, tableFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), dir, serve(t, fixture(t, "likely-old.xml"))); err == nil {
		t.Error("run over a directory where the table stands succeeded, want an error")
	}
}

func TestGenerateRows(t *testing.T) {
	t.Parallel()
	table, err := generate(fixture(t, "likely-old.xml"), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{"ku", scriptLatn}`, `{"ku", "IQ", scriptArab}`, `{"zh", "TW", scriptHant}`, `scriptCyrl = "Cyrl"`} {
		if !bytes.Contains(table, []byte(want)) {
			t.Errorf("the table lacks the row %s", want)
		}
	}
	// und rows and language-script rows resolve nothing a stated tag needs.
	for _, absent := range []string{`"und"`, `{"ku", scriptArab}`} {
		if bytes.Contains(table, []byte(absent)) {
			t.Errorf("the table holds %s, want und and language-script rows dropped", absent)
		}
	}
}

func TestGenerateRefusesMalformed(t *testing.T) {
	t.Parallel()
	for name, rows := range map[string]string{
		"no rows":         ``,
		"short to":        `<likelySubtag from="ku" to="ku_TR"/>`,
		"bad script":      `<likelySubtag from="ku" to="ku_latn_TR"/>`,
		"bad language":    `<likelySubtag from="K1" to="k1_Latn_TR"/>`,
		"three-part from": `<likelySubtag from="ku_Latn_TR" to="ku_Latn_TR"/>`,
		"bad region":      `<likelySubtag from="ku_T" to="ku_Latn_TR"/>`,
		"repeated key":    `<likelySubtag from="ku" to="ku_Latn_TR"/><likelySubtag from="ku" to="ku_Arab_IQ"/>`,
		"repeated region": `<likelySubtag from="ku_IQ" to="ku_Arab_IQ"/><likelySubtag from="ku_IQ" to="ku_Latn_IQ"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := []byte("<supplementalData><likelySubtags>" + rows + "</likelySubtags></supplementalData>")
			if _, err := generate(src, "fixture"); err == nil {
				t.Errorf("generate over %s succeeded, want a refusal", rows)
			}
		})
	}
	t.Run("not xml", func(t *testing.T) {
		t.Parallel()
		if _, err := generate([]byte("<supplementalData><likelySubtags>"), "fixture"); err == nil {
			t.Error("generate over truncated XML succeeded, want a refusal")
		}
	})
}
