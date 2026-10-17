// Package cldr is the one pin langtag's table generators read: the CLDR
// release, its cldr-json release and the SHA-256 of every input file, held
// in cldr.go, which go run ./internal/gen/pin writes. Every generator reads
// its inputs through Read, so every table comes from one release.
package cldr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"

	"github.com/cplieger/langtag/v3/internal/gen/fetch"
)

// Pin is a CLDR release tag and the cldr-json release packaging it.
type Pin struct {
	Release, JSONVersion string
}

// Pinned returns the pin cldr.go holds.
func Pinned() Pin { return Pin{Release: Release, JSONVersion: JSONVersion} }

// origin is where a File is published.
type origin int

const (
	fromCLDR origin = iota
	fromCLDRJSON
	fromISO
)

// File is one input a generator reads.
type File struct {
	// name is the file's path in its repository, which cldr.go keys its
	// SHA-256 by.
	name   string
	origin origin
	// limit bounds the download: each is several times the file's size at
	// release-48.
	limit int
}

// The inputs, each read by one generator.
var (
	LikelySubtags      = File{"common/supplemental/likelySubtags.xml", fromCLDR, 16 << 20}
	SupplementalData   = File{"common/supplemental/supplementalData.xml", fromCLDR, 4 << 20}
	LanguageInfo       = File{"common/supplemental/languageInfo.xml", fromCLDR, 4 << 20}
	Languages          = File{"cldr-json/cldr-localenames-full/main/en/languages.json", fromCLDRJSON, 1 << 20}
	Scripts            = File{"cldr-json/cldr-localenames-full/main/en/scripts.json", fromCLDRJSON, 1 << 20}
	Territories        = File{"cldr-json/cldr-localenames-full/main/en/territories.json", fromCLDRJSON, 1 << 20}
	LocaleDisplayNames = File{"cldr-json/cldr-localenames-full/main/en/localeDisplayNames.json", fromCLDRJSON, 1 << 20}
	ISO6392            = File{"ISO-639-2_utf-8.txt", fromISO, 1 << 20}
)

// Files lists every input in the order cldr.go pins them.
func Files() []File {
	return []File{LikelySubtags, SupplementalData, LanguageInfo, Languages, Scripts, Territories, LocaleDisplayNames, ISO6392}
}

// Name returns f's path in its repository: common/supplemental/languageInfo.xml.
func (f File) Name() string { return f.name }

// Base returns f's file name: languageInfo.xml.
func (f File) Base() string { return path.Base(f.name) }

// Versioned reports whether f moves with the CLDR pin. The ISO 639-2 file
// is published without versions, so it keeps its own SHA-256 and fetch date.
func (f File) Versioned() bool { return f.origin != fromISO }

// URL returns where f is published at p.
func (f File) URL(p Pin) string {
	switch f.origin {
	case fromCLDR:
		return "https://raw.githubusercontent.com/unicode-org/cldr/" + p.Release + "/" + f.name
	case fromCLDRJSON:
		return "https://raw.githubusercontent.com/unicode-org/cldr-json/" + p.JSONVersion + "/" + f.name
	default:
		return "https://www.loc.gov/standards/iso639-2/" + f.name
	}
}

// SHA256 returns the SHA-256 cldr.go pins f to.
func (f File) SHA256() string { return sums[f.name] }

// Read returns f at the pin, refusing a body whose SHA-256 is not the
// pinned one.
func Read(ctx context.Context, f File) ([]byte, error) {
	b, err := Get(ctx, f, Pinned())
	if err != nil {
		return nil, err
	}
	if err := check(f, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Get returns f at p without checking its SHA-256: the bytes of a release
// not pinned yet.
func Get(ctx context.Context, f File, p Pin) ([]byte, error) {
	return fetch.Get(ctx, f.URL(p), nil, f.limit)
}

// check refuses b unless it hashes to f's pinned SHA-256.
func check(f File, b []byte) error {
	if got := SHA256(b); got != f.SHA256() {
		return fmt.Errorf("%s has SHA-256 %s, want the pinned %s", f.Base(), got, f.SHA256())
	}
	return nil
}

// SHA256 returns the hex SHA-256 of b.
func SHA256(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// tagShape keeps a release tag to what CLDR and cldr-json tags use, since a
// tag is spliced into URLs and into cldr.go.
var tagShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CheckTag refuses a release tag outside tagShape.
func CheckTag(tag string) error {
	if !tagShape.MatchString(tag) {
		return fmt.Errorf("release tag %q: want letters, digits, '.', '_' or '-'", tag)
	}
	return nil
}

// jsonShape is a cldr-json release tag, <major>.<minor>.<patch>, where a
// patch repackages one CLDR release.
var jsonShape = regexp.MustCompile(`^(\d+)\.(\d+)\.\d+$`)

// ReleaseOf returns the CLDR release a cldr-json release packages: 48.0.0 is
// release-48 and 48.2.3 is release-48-2.
func ReleaseOf(jsonVersion string) (string, error) {
	m := jsonShape.FindStringSubmatch(jsonVersion)
	if m == nil {
		return "", fmt.Errorf("cldr-json release %q: want <major>.<minor>.<patch>", jsonVersion)
	}
	if m[2] == "0" {
		return "release-" + m[1], nil
	}
	return "release-" + m[1] + "-" + m[2], nil
}
