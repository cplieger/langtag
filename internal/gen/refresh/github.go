package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cplieger/langtag/v3/internal/gen/fetch"
)

type github struct {
	base, token string
}

// perPage and maxPages bound a release listing: cldr-json listed 90
// releases at CLDR 48.2.
const (
	perPage  = 100
	maxPages = 10
)

// latest returns the tag of repo's latest release, which GitHub defines as
// the newest one that is neither a draft nor a pre-release.
func (g github) latest(ctx context.Context, repo string) (string, error) {
	var r struct {
		TagName string `json:"tag_name"`
	}
	if err := g.get(ctx, "/repos/"+repo+"/releases/latest", &r); err != nil {
		return "", err
	}
	if r.TagName == "" {
		return "", fmt.Errorf("%s's latest release names no tag", repo)
	}
	return r.TagName, nil
}

// tags returns the tag of every release of repo that is neither a draft nor
// a pre-release, newest first.
func (g github) tags(ctx context.Context, repo string) ([]string, error) {
	var out []string
	for page := 1; page <= maxPages; page++ {
		var releases []struct {
			TagName    string `json:"tag_name"`
			Draft      bool   `json:"draft"`
			Prerelease bool   `json:"prerelease"`
		}
		path := fmt.Sprintf("/repos/%s/releases?per_page=%d&page=%d", repo, perPage, page)
		if err := g.get(ctx, path, &releases); err != nil {
			return nil, err
		}
		for _, r := range releases {
			if !r.Draft && !r.Prerelease {
				out = append(out, r.TagName)
			}
		}
		if len(releases) < perPage {
			return out, nil
		}
	}
	return nil, fmt.Errorf("%s lists more than %d releases", repo, perPage*maxPages)
}

func (g github) get(ctx context.Context, path string, v any) error {
	header := http.Header{
		"Accept":               {"application/vnd.github+json"},
		"X-Github-Api-Version": {"2022-11-28"},
	}
	if g.token != "" {
		header.Set("Authorization", "Bearer "+g.token)
	}
	// A page of 100 cldr-json releases, notes included, was 0.5 MB.
	body, err := fetch.Get(ctx, g.base+path, header, 8<<20)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
