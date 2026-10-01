package casebook

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

const (
	defaultDelay = 100 * time.Millisecond
	multiply     = 2
	jitter       = 100 * time.Millisecond
)

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func backoff(attempt int) time.Duration {
	delay := defaultDelay
	for range attempt {
		delay *= multiply
	}
	if jitter <= 0 {
		return delay
	}
	return delay + time.Duration(rand.Int63n(int64(jitter)))
}

type retryMiddleware struct {
	next       http.RoundTripper
	maxRetries int
}

func setupRetry(next http.RoundTripper, maxRetries int) http.RoundTripper {
	if maxRetries < 1 {
		maxRetries = 1
	}
	return &retryMiddleware{
		next:       next,
		maxRetries: maxRetries,
	}
}

func (m *retryMiddleware) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()

	var lastErr error
	for attempt := 0; attempt < m.maxRetries; attempt++ {
		resp, err := m.next.RoundTrip(r.Clone(ctx))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < http.StatusBadRequest {
			return resp, nil
		}

		apiErr := unmarshalError(resp.StatusCode, resp.Body)
		lastErr = apiErr
		if !apiErr.IsRetryable() || attempt == m.maxRetries-1 {
			return nil, apiErr
		}
		if err := sleep(ctx, backoff(attempt)); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}
