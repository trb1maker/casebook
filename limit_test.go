package casebook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimitZeroUsesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithLimit(0))
	body, err := c.Get(context.Background(), "/cases")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"name":"ok"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestLimitBurstBlocksSecondRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithLimit(1))
	if _, err := c.Get(context.Background(), "/cases"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Get(ctx, "/cases")
	if err == nil || !strings.Contains(err.Error(), "would exceed context deadline") {
		t.Fatalf("error = %v", err)
	}
}
