package casebook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type seenHeaders struct {
	mu      sync.Mutex
	key     string
	version string
	hits    int
}

func (s *seenHeaders) record(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = r.Header.Get(apiKeyHeader)
	s.version = r.Header.Get(apiVersionHeader)
	s.hits++
}

func (s *seenHeaders) snapshot() (key, version string, hits int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key, s.version, s.hits
}

func TestAuthHeadersOnConfiguredHost(t *testing.T) {
	var seen seenHeaders
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.record(r)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("secret", 2, WithHost(srv.URL))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}

	key, version, hits := seen.snapshot()
	if hits != 1 || key != "secret" || version != "2" {
		t.Fatalf("hits=%d key=%q version=%q", hits, key, version)
	}
}
