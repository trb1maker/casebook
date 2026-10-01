package casebook

import "net/http"

const (
	apiKeyHeader     = "apiKey"
	apiVersionHeader = "version"
)

type authMiddleware struct {
	next       http.RoundTripper
	apiKey     string
	apiVersion string
}

func (m *authMiddleware) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(apiKeyHeader, m.apiKey)
	r.Header.Set(apiVersionHeader, m.apiVersion)
	return m.next.RoundTrip(r) //nolint:wrapcheck // Client.Get wraps the error from Do.
}

func setupAuth(next http.RoundTripper, apiKey, apiVersion string) http.RoundTripper {
	return &authMiddleware{
		next:       next,
		apiKey:     apiKey,
		apiVersion: apiVersion,
	}
}
