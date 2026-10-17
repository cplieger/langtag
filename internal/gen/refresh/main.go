// Command refresh moves langtag's CLDR pin to the latest CLDR release, or
// the one -release names, at the newest cldr-json release packaging it, then
// regenerates every table and the names golden.
//
//	go run ./internal/gen/refresh [-release <tag>] [-out <dir>]
//
// It prints "no change" when the pin already names both releases, and
// otherwise every path it changed. With -out it also writes the pull
// request's title and body there, as title and body.md. It fails when
// cldr-json has not published the CLDR release yet. Run from the module
// root; GITHUB_TOKEN, when set, authenticates the GitHub API reads.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

func main() {
	release := flag.String("release", "", "move to this CLDR release tag (default: the latest CLDR release)")
	out := flag.String("out", "", "when the pin moves, write the pull request's title and body.md to this directory")
	flag.Parse()
	api := github{base: "https://api.github.com", token: os.Getenv("GITHUB_TOKEN")}
	req := request{root: ".", release: *release, out: *out}
	if err := run(context.Background(), os.Stdout, req, api, goCommand); err != nil {
		fmt.Fprintln(os.Stderr, "refresh:", err)
		os.Exit(1)
	}
}

type request struct {
	root, release, out string
}

type goFunc func(ctx context.Context, root string, stdout io.Writer, env []string, args ...string) error

var generated = []string{
	"internal/gen/cldr/cldr.go",
	"scripttable.go",
	"regiontable.go",
	"names/table.go",
	"names/testdata/names.golden",
}

func run(ctx context.Context, w io.Writer, req request, api github, goRun goFunc) error {
	from := cldr.Pinned()
	to, err := resolve(ctx, api, req.release, from)
	if err != nil {
		return err
	}
	if to == from {
		_, err = fmt.Fprintf(w, "no change: the pin already names %s (cldr-json %s)\n", to.Release, to.JSONVersion)
		return err
	}
	before, err := snapshot(req.root)
	if err != nil {
		return err
	}
	report, err := regenerate(ctx, req.root, to, goRun)
	if err != nil {
		return err
	}
	after, err := snapshot(req.root)
	if err != nil {
		return err
	}
	if req.out != "" {
		err = writeText(req.out, title(to), body(from, to, report))
	}
	if err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "moved the pin from %s (cldr-json %s) to %s (cldr-json %s)\n", from.Release, from.JSONVersion, to.Release, to.JSONVersion)
	for _, path := range generated {
		if !bytes.Equal(before[path], after[path]) {
			fmt.Fprintf(&b, "changed %s\n", path)
		}
	}
	_, err = io.WriteString(w, b.String())
	return err
}

// releaseShape is a CLDR release tag: release-48, or release-48-2 for a
// minor release.
var releaseShape = regexp.MustCompile(`^release-\d+(?:-[1-9]\d*)?$`)

// resolve returns the pin for release, or for the latest CLDR release when
// release is "": the release and the newest cldr-json release packaging it.
// It refuses a release cldr-json has not packaged yet, so a pin never mixes
// two releases; pinned names the pin that stays.
func resolve(ctx context.Context, api github, release string, pinned cldr.Pin) (cldr.Pin, error) {
	if release == "" {
		latest, err := api.latest(ctx, "unicode-org/cldr")
		if err != nil {
			return cldr.Pin{}, err
		}
		release = latest
	}
	if !releaseShape.MatchString(release) {
		return cldr.Pin{}, fmt.Errorf("CLDR release tag %q: want release-<major> or release-<major>-<minor>", release)
	}
	tags, err := api.tags(ctx, "unicode-org/cldr-json")
	if err != nil {
		return cldr.Pin{}, err
	}
	newest, newestPatch := "", -1
	for _, tag := range tags {
		if packaged, err := cldr.ReleaseOf(tag); err != nil || packaged != release {
			continue
		}
		// ReleaseOf accepted <major>.<minor>.<patch>, all digits.
		patch, err := strconv.Atoi(tag[strings.LastIndexByte(tag, '.')+1:])
		if err == nil && patch > newestPatch {
			newest, newestPatch = tag, patch
		}
	}
	if newest == "" {
		return cldr.Pin{}, fmt.Errorf("cldr-json has published no release of CLDR %s yet; the pin stays at %s (cldr-json %s)", release, pinned.Release, pinned.JSONVersion)
	}
	return cldr.Pin{Release: release, JSONVersion: newest}, nil
}

// isoHint names the one input a failed regeneration can blame that does not
// move with the CLDR pin.
const isoHint = "the names generator also fetches ISO-639-2_utf-8.txt from www.loc.gov, which keeps its own pin: " +
	"a SHA-256 refusal naming that file means loc.gov edited it, not a CLDR break, and go run ./internal/gen/pin -iso moves it"

// regenerate moves root's pin to to and rewrites every generated file,
// returning the script table's report, which it takes before the pin moves
// because the report compares the pinned release with the new one.
func regenerate(ctx context.Context, root string, to cldr.Pin, goRun goFunc) (string, error) {
	var report bytes.Buffer
	if err := goRun(ctx, root, &report, nil, "run", "./internal/gen/likely", "-diff", to.Release); err != nil {
		return "", fmt.Errorf("report the script table's change: %w", err)
	}
	if err := goRun(ctx, root, os.Stderr, nil, "run", "./internal/gen/pin", "-release", to.Release, "-json", to.JSONVersion); err != nil {
		return "", fmt.Errorf("move the pin: %w", err)
	}
	if err := goRun(ctx, root, os.Stderr, nil, "generate", "./..."); err != nil {
		return "", fmt.Errorf("regenerate the tables: %w; %s", err, isoHint)
	}
	if err := goRun(ctx, root, os.Stderr, []string{"UPDATE_GOLDEN=1"}, "test", "-count=1", "-run", "^TestNamesGolden$", "./names"); err != nil {
		return "", fmt.Errorf("regenerate the names golden: %w", err)
	}
	return report.String(), nil
}

// snapshot returns the content of each generated file under root, nil for
// one that does not exist.
func snapshot(root string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(generated))
	for _, path := range generated {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		out[path] = b
	}
	return out, nil
}

func title(to cldr.Pin) string {
	return fmt.Sprintf("feat: update the CLDR data to %s (cldr-json %s)", to.Release, to.JSONVersion)
}

// bodyLimit keeps a body under GitHub's 65,536-character cap on a pull
// request body with room to spare; the cut falls in the report alone, whose
// every row the regenerated scripttable.go carries anyway.
const bodyLimit = 60000

const cutNote = "\nThe report is cut here; scripttable.go in this diff lists every row.\n"

// body returns the pull request's body for a move from from to to, carrying
// report, likely -diff's Markdown, cut at a line to fit bodyLimit.
func body(from, to cldr.Pin, report string) string {
	head := fmt.Sprintf("Moves the CLDR pin from %#q (cldr-json %#q) to %#q (cldr-json %#q).\n\n", from.Release, from.JSONVersion, to.Release, to.JSONVersion)
	tail := "\nThe English names change, if any, is the diff of names/testdata/names.golden.\n" +
		"If a row the hand copies in regions_test.go, scripttable_test.go or scripts_test.go hold moved, those tests fail until the copies are updated by hand to the reviewed rows, with the release and lines they cite.\n" +
		"A script or region example, or a CLDR release or line citation, this makes false in README.md or docs/ is corrected in this pull request.\n"
	if report == "" {
		report = "The script table keeps every row.\n"
	}
	if room := bodyLimit - len(head) - len(tail); len(report) > room {
		cut := report[:room-len(cutNote)]
		report = cut[:strings.LastIndexByte(cut, '\n')+1] + cutNote
	}
	return head + report + tail
}

func writeText(dir, title, body string) error {
	if err := os.WriteFile(filepath.Join(dir, "title"), []byte(title+"\n"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "body.md"), []byte(body), 0o600)
}

func goCommand(ctx context.Context, root string, stdout io.Writer, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
