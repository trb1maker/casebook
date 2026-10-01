package casebook

import (
	"net/http"

	"golang.org/x/time/rate"
)

const (
	burstDivisor     = 5
	secondsPerMinute = 60
)

type limitMiddleware struct {
	next  http.RoundTripper
	limit *rate.Limiter
}

func setupLimit(next http.RoundTripper, max int) http.RoundTripper {
	if max <= 0 {
		max = defaultLimit
	}
	burst := max / burstDivisor
	if burst < 1 {
		burst = 1
	}

	return &limitMiddleware{
		next:  next,
		limit: rate.NewLimiter(rate.Limit(float64(max)/secondsPerMinute), burst),
	}
}

func (m *limitMiddleware) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := m.limit.Wait(r.Context()); err != nil {
		return nil, err
	}
	return m.next.RoundTrip(r)
}
