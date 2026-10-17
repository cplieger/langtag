package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

func TestMultiScriptDiffNamesNewLanguage(t *testing.T) {
	t.Parallel()
	pt := row{language: "pt", script: "Latn"}
	ptMO := row{language: "pt", region: "MO", script: "Hant"}
	ku := row{language: "ku", script: "Latn"}
	kuAM := row{language: "ku", region: "AM", script: "Cyrl"}
	kuIQ := row{language: "ku", region: "IQ", script: "Arab"}
	zhTW := row{language: "zh", region: "TW", script: "Hant"}
	for _, tc := range []struct {
		name      string
		old, next []row
		want      []string
	}{
		{
			name: "pt gains a region row",
			old:  []row{ku, kuIQ, pt},
			next: []row{ku, kuIQ, pt, ptMO},
			want: []string{"Count: 2 before, 3 after.", "- Entering: `pt`\n", "- Leaving: none\n", "changed rows (1)", "| `pt_MO` | none | `Hant` |\n"},
		},
		{
			// kmr has no row of its own and is MultiScript through ku.
			name: "ku loses its region rows",
			old:  []row{ku, kuAM, kuIQ, pt},
			next: []row{ku, pt},
			want: []string{"Count: 2 before, 0 after.", "- Entering: none\n", "- Leaving: `kmr`, `ku`\n", "changed rows (2)", "| `ku_AM` | `Cyrl` | none |\n", "| `ku_IQ` | `Arab` | none |\n"},
		},
		{
			name: "a row changes script",
			old:  []row{zhTW},
			next: []row{{language: "zh", region: "TW", script: "Hans"}},
			want: []string{"- Entering: none\n", "- Leaving: none\n", "| `zh_TW` | `Hant` | `Hans` |\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			if err := writeDiff(&b, tc.old, tc.next); err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("the report lacks %q:\n%s", w, b.String())
				}
			}
		})
	}
	t.Run("equal tables", func(t *testing.T) {
		t.Parallel()
		var b bytes.Buffer
		if err := writeDiff(&b, []row{ku, kuIQ, pt}, []row{pt, kuIQ, ku}); err != nil {
			t.Fatal(err)
		}
		if b.Len() != 0 {
			t.Errorf("the report over equal tables is %q, want nothing", b.String())
		}
	})
}

// releases serves the pinned likelySubtags.xml as old and release
// "new-fixture"'s as next, failing the test on any other fetch.
func releases(t *testing.T, old, next []byte) (readFunc, getFunc) {
	t.Helper()
	get := func(_ context.Context, f cldr.File, p cldr.Pin) ([]byte, error) {
		if f != cldr.LikelySubtags || p.Release != "new-fixture" {
			t.Errorf("fetched %s at %+v, want likelySubtags.xml at new-fixture", f.Name(), p)
		}
		return next, nil
	}
	return serve(t, old), get
}

// TestReportOldToNew checks the report compares the pinned release with the
// new one, in memory: the baseline is the pin, whatever the tree holds.
func TestReportOldToNew(t *testing.T) {
	t.Parallel()
	read, get := releases(t, fixture(t, "likely-old.xml"), fixture(t, "likely-new.xml"))
	var b bytes.Buffer
	if err := report(t.Context(), &b, "new-fixture", read, get); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- Entering: `pt`\n", "| `pt_MO` | none | `Hant` |\n"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, b.String())
		}
	}
}

// TestReportEmptyAtUnchangedTable checks a release keeping every row writes
// nothing, which the workflow reads as an empty file.
func TestReportEmptyAtUnchangedTable(t *testing.T) {
	t.Parallel()
	src := fixture(t, "likely-new.xml")
	read, get := releases(t, src, src)
	var b bytes.Buffer
	if err := report(t.Context(), &b, "new-fixture", read, get); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 0 {
		t.Errorf("the report at an unchanged table is %q, want nothing", b.String())
	}
}

// TestReportWriteFailure checks a report that cannot be written fails the
// run: the workflow then opens no pull request without its report.
func TestReportWriteFailure(t *testing.T) {
	t.Parallel()
	read, get := releases(t, fixture(t, "likely-old.xml"), fixture(t, "likely-new.xml"))
	errWrite := errors.New("stdout closed")
	if err := report(t.Context(), failingWriter{errWrite}, "new-fixture", read, get); !errors.Is(err, errWrite) {
		t.Errorf("report with an unwritable output = %v, want the write error", err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// TestReportFetchFailureWritesNothing checks a release that cannot be read
// reports nothing.
func TestReportFetchFailureWritesNothing(t *testing.T) {
	t.Parallel()
	errFetch := errors.New("fetch: 404 Not Found")
	get := func(context.Context, cldr.File, cldr.Pin) ([]byte, error) { return nil, errFetch }
	var b bytes.Buffer
	if err := report(t.Context(), &b, "new-fixture", serve(t, fixture(t, "likely-old.xml")), get); !errors.Is(err, errFetch) {
		t.Errorf("report over a failed fetch = %v, want %v", err, errFetch)
	}
	if b.Len() != 0 {
		t.Errorf("report wrote %q though the fetch failed", b.String())
	}
}

// TestReportRefusesUnsafeTag pins the check on the tag, which is spliced
// into the fetched URL: report refuses it before any fetch.
func TestReportRefusesUnsafeTag(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"../main", "release 48", `release-48"`, "-release"} {
		read := func(context.Context, cldr.File) ([]byte, error) {
			t.Errorf("report(%q) read the pin", tag)
			return nil, errors.New("no fetch expected")
		}
		get := func(context.Context, cldr.File, cldr.Pin) ([]byte, error) {
			t.Errorf("report(%q) fetched", tag)
			return nil, errors.New("no fetch expected")
		}
		err := report(t.Context(), &bytes.Buffer{}, tag, read, get)
		if err == nil || !strings.Contains(err.Error(), "release tag") {
			t.Errorf("report(%q) = %v, want the release tag refusal", tag, err)
		}
	}
}
