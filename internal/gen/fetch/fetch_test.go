package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGet covers Get's refusals: a status other than 200, and a body over the
// limit, refused once limit+1 bytes arrive rather than after the server stops
// sending.
func TestGet(t *testing.T) {
	t.Parallel()
	const limit = 1 << 10
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/max":
			_, _ = w.Write(make([]byte, limit))
		case "/endless":
			_, _ = w.Write(make([]byte, limit+1))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		if _, err := Get(t.Context(), srv.URL+"/missing", limit); err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("Get of a 404 = %v, want an error naming the status", err)
		}
	})
	t.Run("at the bound", func(t *testing.T) {
		t.Parallel()
		body, err := Get(t.Context(), srv.URL+"/max", limit)
		if err != nil || len(body) != limit {
			t.Errorf("Get of %d bytes = %d bytes, %v; want them all", limit, len(body), err)
		}
	})
	t.Run("over the bound", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		_, err := Get(ctx, srv.URL+"/endless", limit)
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Errorf("Get of an endless body = %v, want the size refusal", err)
		}
	})
}
