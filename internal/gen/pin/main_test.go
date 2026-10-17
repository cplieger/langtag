package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

func pinnedSums() map[string]string {
	out := map[string]string{}
	for _, f := range cldr.Files() {
		out[f.Name()] = f.SHA256()
	}
	return out
}

// TestRenderIsCheckedIn checks cldr.go is exactly what this command writes
// for the values it holds, so a pin move rewrites the whole file and changes
// only those values.
func TestRenderIsCheckedIn(t *testing.T) {
	t.Parallel()
	want, err := render(cldr.Pinned(), cldr.ISOFetched, pinnedSums())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "cldr", "cldr.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("internal/gen/cldr/cldr.go differs from what go run ./internal/gen/pin writes for its own pin:\n%s", want)
	}
}

// fakeGet serves "<name> at <release or cldr-json version>" as each file's
// body and counts the fetches.
type fakeGet struct {
	fetched []string
	fail    string
}

func (g *fakeGet) get(_ context.Context, f cldr.File, p cldr.Pin) ([]byte, error) {
	g.fetched = append(g.fetched, f.Base())
	if f.Base() == g.fail {
		return nil, fmt.Errorf("fetch %s: 404 Not Found", f.URL(p))
	}
	return []byte(f.Name() + " at " + p.Release + " " + p.JSONVersion), nil
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(filepath.Dir(pinFile))), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunMovesTheCLDRPin(t *testing.T) {
	t.Parallel()
	root, g := moduleRoot(t), &fakeGet{}
	if err := run(t.Context(), root, request{release: "release-49", jsonVersion: "49.0.0", today: "2027-01-02"}, g.get); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pinFile)))
	if err != nil {
		t.Fatal(err)
	}
	next := cldr.Pin{Release: "release-49", JSONVersion: "49.0.0"}
	want := pinnedSums()
	for _, f := range cldr.Files() {
		if f.Versioned() {
			want[f.Name()] = cldr.SHA256([]byte(f.Name() + " at release-49 49.0.0"))
		}
	}
	wantSrc, err := render(next, cldr.ISOFetched, want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantSrc) {
		t.Errorf("the pin after -release release-49 -json 49.0.0 is:\n%s\nwant:\n%s", got, wantSrc)
	}
	if strings.Contains(strings.Join(g.fetched, " "), cldr.ISO6392.Base()) {
		t.Errorf("-release fetched %v, want the ISO 639-2 file kept at its pin", g.fetched)
	}
}

func TestRunISOKeepsTheCLDRPin(t *testing.T) {
	t.Parallel()
	root, g := moduleRoot(t), &fakeGet{}
	if err := run(t.Context(), root, request{iso: true, today: "2027-01-02"}, g.get); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pinFile)))
	if err != nil {
		t.Fatal(err)
	}
	want := pinnedSums()
	want[cldr.ISO6392.Name()] = cldr.SHA256([]byte(cldr.ISO6392.Name() + " at " + cldr.Release + " " + cldr.JSONVersion))
	wantSrc, err := render(cldr.Pinned(), "2027-01-02", want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantSrc) {
		t.Errorf("the pin after -iso is:\n%s\nwant:\n%s", got, wantSrc)
	}
	if strings.Join(g.fetched, " ") != cldr.ISO6392.Base() {
		t.Errorf("-iso fetched %v, want only the ISO 639-2 file", g.fetched)
	}
}

// TestRunRefuses checks every refusal writes no pin.
func TestRunRefuses(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		req     request
		fail    string
		wantErr string
	}{
		"nothing to move":        {request{}, "", "nothing to move"},
		"no cldr-json release":   {request{release: "release-49"}, "", "-release needs -json"},
		"cldr-json alone":        {request{iso: true, jsonVersion: "49.0.0"}, "", "-json needs -release"},
		"another release's json": {request{release: "release-49", jsonVersion: "48.0.0"}, "", "cldr-json 48.0.0 packages CLDR release-48, not release-49"},
		"minor of a release":     {request{release: "release-49", jsonVersion: "49.1.0"}, "", "packages CLDR release-49-1"},
		"unsafe release":         {request{release: "release-49\"", jsonVersion: "49.0.0"}, "", "release tag"},
		"unsafe json":            {request{release: "release-49", jsonVersion: "49.0.0/.."}, "", "release tag"},
		"cldr-json lacks it":     {request{release: "release-49", jsonVersion: "49.0.0"}, "scripts.json", "404"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := moduleRoot(t)
			err := run(t.Context(), root, c.req, (&fakeGet{fail: c.fail}).get)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("run = %v, want an error containing %q", err, c.wantErr)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(pinFile))); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("run wrote %s though it refused (stat: %v)", pinFile, err)
			}
		})
	}
}
