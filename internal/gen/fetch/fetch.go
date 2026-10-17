// Package fetch downloads the CLDR files langtag's generators read.
package fetch

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
)

// Get returns the body at url, requested with header, refusing a status
// other than 200 and a body over limit bytes, which it stops reading once
// limit+1 bytes arrive.
func Get(ctx context.Context, url string, header http.Header, limit int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	maps.Copy(req.Header, header)
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if len(body) > limit {
		return nil, fmt.Errorf("fetch %s: larger than %d bytes", url, limit)
	}
	return body, nil
}
