package casebook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type item struct {
	Name string `json:"name"`
}

func TestGetDecodesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cases" {
			http.Error(w, "unexpected path", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"case"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "case" {
		t.Fatalf("name = %q", out.Name)
	}
}

func TestWithHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"remote"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	if c.host != srv.URL {
		t.Fatalf("host = %s", c.host)
	}

	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "remote" {
		t.Fatalf("name = %q", out.Name)
	}
}

func TestEmptyHostRestoresDefault(t *testing.T) {
	c := NewClient("k", 1, WithHost(""))
	if c.host != defaultHost {
		t.Fatalf("host = %q", c.host)
	}
}

func TestGetRejectsBadHost(t *testing.T) {
	c := NewClient("k", 1, WithHost("http://["))
	var out item
	err := c.Get(context.Background(), "/cases", &out)
	if err == nil || !strings.Contains(err.Error(), "request:") {
		t.Fatalf("error = %v", err)
	}
}

func TestWithTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithTimeout(20*time.Millisecond))
	if c.http.Timeout != 20*time.Millisecond {
		t.Fatalf("timeout = %s", c.http.Timeout)
	}

	var out item
	err := c.Get(context.Background(), "/cases", &out)
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestWithIdleConns(t *testing.T) {
	const idle = 4
	c := NewClient("k", 1, WithIdleConns(idle))
	transport := c.http.Transport.(*authMiddleware).next.(*retryMiddleware).next.(*limitMiddleware).next.(*http.Transport)
	if transport.MaxIdleConnsPerHost != idle {
		t.Fatalf("idle conns = %d", transport.MaxIdleConnsPerHost)
	}
}

func TestGetBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "not-json")
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	err := c.Get(context.Background(), "/cases", &out)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("error = %v", err)
	}
}

func TestCanceledContextDoesNotRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"case"}`)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	err := c.Get(ctx, "/cases", &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("hits = %d", hits.Load())
	}
}
