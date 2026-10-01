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

func TestSleepReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sleep(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestRetryStopsWhenContextCanceledDuringBackoff(t *testing.T) {
	var hits atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		cancel()
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"details":["temporary"]}`)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(cancel)

	c := NewClient("k", 1, WithHost(srv.URL), WithMaxRetries(3))
	var out item
	err := c.Get(ctx, "/cases", &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestRetryReturnsContextErrorDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var hits atomic.Int32
	next := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits.Add(1)
		time.AfterFunc(10*time.Millisecond, cancel)
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{"details":["temporary"]}`)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/cases", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = setupRetry(next, 3).RoundTrip(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestBackoff(t *testing.T) {
	bases := []time.Duration{
		defaultDelay,
		defaultDelay * multiply,
		defaultDelay * multiply * multiply,
	}
	for attempt, base := range bases {
		for range 20 {
			got := backoff(attempt)
			if got < base || got >= base+jitter {
				t.Fatalf("attempt %d: %s not in [%s, %s)", attempt, got, base, base+jitter)
			}
		}
	}
}

func TestRetryEventuallySucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"details":["temporary"]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "ok" {
		t.Fatalf("name = %q", out.Name)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestRetrySkipsClientErrors(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusNotFound} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(code)
				io.WriteString(w, `{"details":["nope"]}`)
			}))
			t.Cleanup(srv.Close)

			c := NewClient("k", 1, WithHost(srv.URL))
			var out item
			err := c.Get(context.Background(), "/cases", &out)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v", err)
			}
			if apiErr.code != code || apiErr.message != "nope" || apiErr.IsRetryable() {
				t.Fatalf("api error = %+v", apiErr)
			}
			if hits.Load() != 1 {
				t.Fatalf("hits = %d", hits.Load())
			}
		})
	}
}

func TestRetryOnTooManyRequests(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"details":["slow"]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestRetryNonJSONServerError(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "unavailable")
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL))
	var out item
	err := c.Get(context.Background(), "/cases", &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v", err)
	}
	if apiErr.code != http.StatusServiceUnavailable || apiErr.message != "unavailable" {
		t.Fatalf("code=%d message=%q", apiErr.code, apiErr.message)
	}
	if hits.Load() != int32(defaultMaxRetries) {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestMaxRetriesBelowOne(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"details":["down"]}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithMaxRetries(0))
	var out item
	err := c.Get(context.Background(), "/cases", &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v", err)
	}
	if apiErr.code != http.StatusInternalServerError {
		t.Fatalf("code = %d", apiErr.code)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestRetryTransportError(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		conn.Close()
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithMaxRetries(3))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err == nil {
		t.Fatal("expected transport error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}
