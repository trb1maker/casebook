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
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "ok" {
		t.Fatalf("name = %q", out.Name)
	}
}

func TestLimitBurstBlocksSecondRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithLimit(1))
	var out item
	if err := c.Get(context.Background(), "/cases", &out); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.Get(ctx, "/cases", &out)
	if err == nil || !strings.Contains(err.Error(), "would exceed context deadline") {
		t.Fatalf("error = %v", err)
	}
}
