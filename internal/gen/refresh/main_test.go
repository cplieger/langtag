package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// A fakeGitHub whose latest is "" fails the test if the latest release is read.
type fakeGitHub struct {
	t        *testing.T
	latest   string
	releases []release
	status   int

	mu    sync.Mutex
	auths []string
}

func (f *fakeGitHub) serve() github {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auths = append(f.auths, r.Header.Get("Authorization"))
		f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		switch r.URL.Path {
		case "/repos/unicode-org/cldr/releases/latest":
			if f.latest == "" {
				f.t.Error("read the latest CLDR release, want the -release tag used")
			}
			_ = json.NewEncoder(w).Encode(release{TagName: f.latest})
		case "/repos/unicode-org/cldr-json/releases":
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err != nil || r.URL.Query().Get("per_page") != strconv.Itoa(perPage) {
				f.t.Errorf("listed %s, want per_page=%d and a page number", r.URL, perPage)
			}
			lo := min((page-1)*perPage, len(f.releases))
			_ = json.NewEncoder(w).Encode(f.releases[lo:min(lo+perPage, len(f.releases))])
		default:
			http.NotFound(w, r)
		}
	}))
	f.t.Cleanup(srv.Close)
	return github{base: srv.URL, token: "test-token"}
}

func published(tags ...string) []release {
	out := make([]release, len(tags))
	for i, tag := range tags {
		out[i] = release{TagName: tag}
	}
	return out
}

type fakeGo struct {
	report string
	writes map[string][]string
	fail   string
	calls  []string
}

func (g *fakeGo) run(_ context.Context, root string, stdout io.Writer, env []string, args ...string) error {
	call := strings.Join(append(slices.Clone(env), args...), " ")
	g.calls = append(g.calls, call)
	step := args[0]
	if args[0] == "run" {
		step = args[1]
	}
	if step == g.fail {
		return fmt.Errorf("go %s: exit status 1", strings.Join(args, " "))
	}
	if step == "./internal/gen/likely" {
		_, err := io.WriteString(stdout, g.report)
		return err
	}
	for _, path := range g.writes[step] {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("new "+path), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range generated {
		p := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("old "+path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func movingGo() *fakeGo {
	return &fakeGo{
		report: "### Script table: changed rows (1)\n",
		writes: map[string][]string{
			"./internal/gen/pin": {"internal/gen/cldr/cldr.go"},
			"generate":           {"scripttable.go", "names/table.go"},
			"test":               {"names/testdata/names.golden"},
		},
	}
}

func readOut(t *testing.T, dir string) (title, body string) {
	t.Helper()
	tb, err := os.ReadFile(filepath.Join(dir, "title"))
	if err != nil {
		t.Fatal(err)
	}
	bb, err := os.ReadFile(filepath.Join(dir, "body.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(tb), string(bb)
}

func assertNoOut(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("wrote %v to -out, want nothing", entries)
	}
}

func TestRunAtThePinChangesNothing(t *testing.T) {
	t.Parallel()
	api := (&fakeGitHub{t: t, latest: cldr.Release, releases: published(cldr.JSONVersion, "47.0.0")}).serve()
	root, out, g := moduleRoot(t), t.TempDir(), movingGo()
	var w bytes.Buffer
	if err := run(t.Context(), &w, request{root: root, out: out}, api, g.run); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("no change: the pin already names %s (cldr-json %s)\n", cldr.Release, cldr.JSONVersion); w.String() != want {
		t.Errorf("printed %q, want %q", w.String(), want)
	}
	if len(g.calls) != 0 {
		t.Errorf("ran %q, want no go command", g.calls)
	}
	assertNoOut(t, out)
}

func TestRunMovesThePin(t *testing.T) {
	t.Parallel()
	fake := &fakeGitHub{t: t, latest: "release-49", releases: []release{
		{TagName: "49.0.3", Draft: true},
		{TagName: "49.0.2", Prerelease: true},
		{TagName: "49.1.0"},
		{TagName: "49.0.1"},
		{TagName: "49.0.0-BETA4", Prerelease: true},
		{TagName: "49.0.0"},
		{TagName: "48.0.0"},
	}}
	api := fake.serve()
	root, out, g := moduleRoot(t), t.TempDir(), movingGo()
	var w bytes.Buffer
	if err := run(t.Context(), &w, request{root: root, out: out}, api, g.run); err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{
		"run ./internal/gen/likely -diff release-49",
		"run ./internal/gen/pin -release release-49 -json 49.0.1",
		"generate ./...",
		"UPDATE_GOLDEN=1 test -count=1 -run ^TestNamesGolden$ ./names",
	}
	if !slices.Equal(g.calls, wantCalls) {
		t.Errorf("ran %q, want %q", g.calls, wantCalls)
	}
	wantOut := fmt.Sprintf("moved the pin from %s (cldr-json %s) to release-49 (cldr-json 49.0.1)\n", cldr.Release, cldr.JSONVersion) +
		"changed internal/gen/cldr/cldr.go\nchanged scripttable.go\nchanged names/table.go\nchanged names/testdata/names.golden\n"
	if w.String() != wantOut {
		t.Errorf("printed %q, want %q", w.String(), wantOut)
	}
	gotTitle, gotBody := readOut(t, out)
	if gotTitle != "feat: update the CLDR data to release-49 (cldr-json 49.0.1)\n" {
		t.Errorf("title = %q", gotTitle)
	}
	if want := body(cldr.Pinned(), cldr.Pin{Release: "release-49", JSONVersion: "49.0.1"}, g.report); gotBody != want {
		t.Errorf("body.md = %q, want %q", gotBody, want)
	}
	for _, auth := range fake.auths {
		if auth != "Bearer test-token" {
			t.Errorf("a GitHub read sent Authorization %q, want the token", auth)
		}
	}
}

func TestRunTakesTheReleaseFlag(t *testing.T) {
	t.Parallel()
	api := (&fakeGitHub{t: t, releases: published("49.0.0", "48.2.3", "48.2.0", "48.1.0", cldr.JSONVersion)}).serve()
	root, g := moduleRoot(t), movingGo()
	var w bytes.Buffer
	if err := run(t.Context(), &w, request{root: root, release: "release-48-2"}, api, g.run); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.calls, "run ./internal/gen/pin -release release-48-2 -json 48.2.3") {
		t.Errorf("ran %q, want the pin moved to release-48-2 at cldr-json 48.2.3", g.calls)
	}
}

func TestRunRefuses(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		latest, release string
		releases        []release
		status          int
		fail            string
		wantErr         string
		wantCalls       int
	}{
		"cldr-json lacks the release": {
			latest: "release-49", releases: published("48.2.3", cldr.JSONVersion),
			wantErr: fmt.Sprintf("cldr-json has published no release of CLDR release-49 yet; the pin stays at %s (cldr-json %s)", cldr.Release, cldr.JSONVersion),
		},
		"only a pre-release of it": {
			latest: "release-49", releases: []release{{TagName: "49.0.0", Prerelease: true}},
			wantErr: "no release of CLDR release-49",
		},
		"a malformed latest tag":  {latest: "release-49-beta1", wantErr: `CLDR release tag "release-49-beta1"`},
		"a malformed release tag": {release: "release-49/../x", wantErr: "want release-<major>"},
		"the API refuses":         {status: http.StatusForbidden, wantErr: "403"},
		"the report fails": {
			latest: "release-49", releases: published("49.0.0"), fail: "./internal/gen/likely",
			wantErr: "report the script table's change", wantCalls: 1,
		},
		"the pin fails": {
			latest: "release-49", releases: published("49.0.0"), fail: "./internal/gen/pin",
			wantErr: "move the pin", wantCalls: 2,
		},
		"the tables fail": {
			latest: "release-49", releases: published("49.0.0"), fail: "generate",
			wantErr: "regenerate the tables: go generate ./...: exit status 1; the names generator also fetches ISO-639-2_utf-8.txt", wantCalls: 3,
		},
		"the golden fails": {
			latest: "release-49", releases: published("49.0.0"), fail: "test",
			wantErr: "regenerate the names golden", wantCalls: 4,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			latest := c.latest
			if c.release != "" {
				latest = ""
			}
			api := (&fakeGitHub{t: t, latest: latest, releases: c.releases, status: c.status}).serve()
			out, g := t.TempDir(), movingGo()
			g.fail = c.fail
			err := run(t.Context(), io.Discard, request{root: moduleRoot(t), release: c.release, out: out}, api, g.run)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("run = %v, want an error containing %q", err, c.wantErr)
			}
			if len(g.calls) != c.wantCalls {
				t.Errorf("ran %q, want %d go commands", g.calls, c.wantCalls)
			}
			assertNoOut(t, out)
		})
	}
}

func TestTagsReadsEveryPage(t *testing.T) {
	t.Parallel()
	releases := make([]release, perPage, perPage+1)
	for i := range releases {
		releases[i] = release{TagName: fmt.Sprintf("49.0.0-ALPHA%d", i), Prerelease: true}
	}
	releases = append(releases, release{TagName: "47.0.0"})
	api := (&fakeGitHub{t: t, releases: releases}).serve()
	tags, err := api.tags(t.Context(), "unicode-org/cldr-json")
	if err != nil || !slices.Equal(tags, []string{"47.0.0"}) {
		t.Errorf("tags = %q, %v; want the one release on the second page", tags, err)
	}
}

func TestTagsRefusesAnEndlessListing(t *testing.T) {
	t.Parallel()
	api := (&fakeGitHub{t: t, releases: published(slices.Repeat([]string{"48.0.0"}, perPage*maxPages)...)}).serve()
	if _, err := api.tags(t.Context(), "unicode-org/cldr-json"); err == nil || !strings.Contains(err.Error(), "more than 1000 releases") {
		t.Errorf("tags of %d full pages = %v, want the cap refusal", maxPages, err)
	}
}

func TestBody(t *testing.T) {
	t.Parallel()
	from, to := cldr.Pin{Release: "release-48", JSONVersion: "48.0.0"}, cldr.Pin{Release: "release-49", JSONVersion: "49.0.1"}
	const reminder = "is corrected in this pull request.\n"
	t.Run("unchanged table", func(t *testing.T) {
		t.Parallel()
		b := body(from, to, "")
		if !strings.HasPrefix(b, "Moves the CLDR pin from `release-48` (cldr-json `48.0.0`) to `release-49` (cldr-json `49.0.1`).\n\nThe script table keeps every row.\n") {
			t.Errorf("body = %q, want the move then the unchanged table", b)
		}
	})
	t.Run("cut", func(t *testing.T) {
		t.Parallel()
		var report strings.Builder
		for i := 0; report.Len() < 2*bodyLimit; i++ {
			fmt.Fprintf(&report, "| `row%d` | `Latn` | `Cyrl` |\n", i)
		}
		b := body(from, to, report.String())
		if len(b) > bodyLimit {
			t.Errorf("body is %d bytes, want at most %d", len(b), bodyLimit)
		}
		before, after, ok := strings.Cut(b, cutNote)
		if !ok || !strings.HasSuffix(after, reminder) {
			t.Fatalf("body ends %q, want the cut note then the reminders", b[max(0, len(b)-400):])
		}
		last := before[strings.LastIndexByte(before[:len(before)-1], '\n')+1:]
		if !strings.HasPrefix(last, "| `row") || !strings.HasSuffix(last, "| `Cyrl` |\n") {
			t.Errorf("the report's last line before the cut is %q, want a whole row", last)
		}
	})
	// room is the longest report that fits whole.
	room := bodyLimit - (len(body(from, to, "\n")) - 1)
	t.Run("at the limit", func(t *testing.T) {
		t.Parallel()
		b := body(from, to, strings.Repeat("y", room-1)+"\n")
		if len(b) != bodyLimit || strings.Contains(b, cutNote) {
			t.Errorf("a body of exactly %d bytes is %d bytes, cut %v; want it whole", bodyLimit, len(b), strings.Contains(b, cutNote))
		}
	})
	t.Run("one over", func(t *testing.T) {
		t.Parallel()
		b := body(from, to, strings.Repeat("y", room)+"\n")
		if len(b) > bodyLimit || !strings.Contains(b, cutNote) {
			t.Errorf("a body one byte over is %d bytes, cut %v; want it cut to fit", len(b), strings.Contains(b, cutNote))
		}
	})
}

func TestGeneratedListsEveryOutput(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(b, []byte("// Code generated by go run ")) {
			rel, err := filepath.Rel(root, path)
			found = append(found, filepath.ToSlash(rel))
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := slices.DeleteFunc(slices.Clone(generated), func(p string) bool { return !strings.HasSuffix(p, ".go") })
	slices.Sort(found)
	slices.Sort(want)
	if !slices.Equal(found, want) {
		t.Errorf("the generated Go files are %q, generated lists %q", found, want)
	}
	if _, err := os.Stat(filepath.Join(root, "names", "testdata", "names.golden")); err != nil {
		t.Errorf("generated lists the names golden: %v", err)
	}
}

func TestGoCommand(t *testing.T) {
	t.Parallel()
	t.Run("output and environment", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		if err := goCommand(t.Context(), t.TempDir(), &out, []string{"GOFLAGS=-mod=mod"}, "env", "GOFLAGS"); err != nil {
			t.Fatal(err)
		}
		if out.String() != "-mod=mod\n" {
			t.Errorf("go env GOFLAGS printed %q, want the added environment's -mod=mod", out.String())
		}
	})
	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		err := goCommand(t.Context(), t.TempDir(), io.Discard, nil, "no-such-command")
		var exit interface{ ExitCode() int }
		if err == nil || !strings.HasPrefix(err.Error(), "go no-such-command: ") || !errors.As(err, &exit) {
			t.Errorf("goCommand of an unknown command = %v, want the exit error naming it", err)
		}
	})
}
