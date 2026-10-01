package casebook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorFields(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		body  string
		msg   string
		retry bool
	}{
		{name: "too many requests", code: 429, body: `{"details":["slow"]}`, msg: "slow", retry: true},
		{name: "server error", code: 500, body: `{"details":["a","b"]}`, msg: "a, b", retry: true},
		{name: "client error", code: 400, body: `{"details":["nope"]}`, msg: "nope", retry: false},
		{name: "empty body", code: 500, body: "", msg: "", retry: true},
		{name: "plain text", code: 503, body: "unavailable", msg: "unavailable", retry: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unmarshalError(tt.code, io.NopCloser(strings.NewReader(tt.body)))
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v", err)
			}
			if apiErr.code != tt.code || apiErr.message != tt.msg || apiErr.IsRetryable() != tt.retry {
				t.Fatalf("code=%d message=%q retry=%v", apiErr.code, apiErr.message, apiErr.IsRetryable())
			}
		})
	}
}

func TestAPIErrorNilBody(t *testing.T) {
	err := unmarshalError(http.StatusBadGateway, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	if apiErr.code != http.StatusBadGateway || apiErr.message != "" {
		t.Fatalf("code=%d message=%q", apiErr.code, apiErr.message)
	}
}

func TestAPIErrorRawMessageIsTruncated(t *testing.T) {
	body := strings.Repeat("x", maxRawError+40)
	err := unmarshalError(http.StatusBadGateway, io.NopCloser(strings.NewReader(body)))
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	if apiErr.message != strings.Repeat("x", maxRawError) {
		t.Fatalf("message length = %d", len(apiErr.message))
	}
}

func TestGetWrapsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"details":["bad"]}`)
	}))
	t.Cleanup(srv.Close)

	c := NewClient("k", 1, WithHost(srv.URL), WithMaxRetries(1))
	_, err := c.Get(context.Background(), "/cases")
	if err == nil || !strings.Contains(err.Error(), "get:") {
		t.Fatalf("error = %v", err)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v", err)
	}
	if apiErr.code != http.StatusBadRequest || apiErr.message != "bad" || apiErr.IsRetryable() {
		t.Fatalf("code=%d message=%q retry=%v", apiErr.code, apiErr.message, apiErr.IsRetryable())
	}
}
