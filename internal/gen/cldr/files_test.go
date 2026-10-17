package cldr

import (
	"regexp"
	"strings"
	"testing"
)

// TestEveryFilePinned checks each input has its own name and a SHA-256 in
// cldr.go, and that the pin holds one release.
func TestEveryFilePinned(t *testing.T) {
	t.Parallel()
	hexSum := regexp.MustCompile(`^[0-9a-f]{64}$`)
	seen := map[string]bool{}
	for _, f := range Files() {
		if seen[f.Name()] {
			t.Errorf("two files are named %s", f.Name())
		}
		seen[f.Name()] = true
		if !hexSum.MatchString(f.SHA256()) {
			t.Errorf("%s is pinned to %q, want a SHA-256", f.Name(), f.SHA256())
		}
	}
	if len(sums) != len(seen) {
		t.Errorf("cldr.go pins %d files, want the %d of Files", len(sums), len(seen))
	}
	if r, err := ReleaseOf(JSONVersion); err != nil || r != Release {
		t.Errorf("JSONVersion %s packages %q (%v), want Release %s", JSONVersion, r, err, Release)
	}
}

func TestCheckRefusesUnpinned(t *testing.T) {
	t.Parallel()
	err := check(LanguageInfo, []byte("<supplementalData/>"))
	if err == nil || !strings.Contains(err.Error(), "want the pinned "+LanguageInfo.SHA256()) {
		t.Errorf("check over bytes other than the pinned file's = %v, want the pin refusal", err)
	}
}

func TestURL(t *testing.T) {
	t.Parallel()
	p := Pin{Release: "release-49", JSONVersion: "49.0.0"}
	for f, want := range map[File]string{
		LikelySubtags: "https://raw.githubusercontent.com/unicode-org/cldr/release-49/common/supplemental/likelySubtags.xml",
		Scripts:       "https://raw.githubusercontent.com/unicode-org/cldr-json/49.0.0/cldr-json/cldr-localenames-full/main/en/scripts.json",
		ISO6392:       "https://www.loc.gov/standards/iso639-2/ISO-639-2_utf-8.txt",
	} {
		if got := f.URL(p); got != want {
			t.Errorf("%s.URL = %s, want %s", f.Base(), got, want)
		}
	}
}

func TestReleaseOf(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"48.0.0": "release-48", "48.2.3": "release-48-2", "49.1.0": "release-49-1"} {
		if got, err := ReleaseOf(in); err != nil || got != want {
			t.Errorf("ReleaseOf(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"49.0.0-BETA4", "48.0", "v48.0.0", ""} {
		if got, err := ReleaseOf(in); err == nil {
			t.Errorf("ReleaseOf(%q) = %q, want an error", in, got)
		}
	}
}

// TestCheckTag pins the check on a tag spliced into URLs and cldr.go.
func TestCheckTag(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"../main", "release 48", `release-48"`, "-release", "release-48\n//go:build ignore", ""} {
		if err := CheckTag(tag); err == nil {
			t.Errorf("CheckTag(%q) = nil, want a refusal", tag)
		}
	}
	for _, tag := range []string{"release-48", "release-48-2", "48.2.3"} {
		if err := CheckTag(tag); err != nil {
			t.Errorf("CheckTag(%q) = %v", tag, err)
		}
	}
}
