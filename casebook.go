package casebook

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	http *http.Client
	host string
}

func NewClient(apiKey string, version int, opts ...Option) *Client {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.host == "" {
		cfg.host = defaultHost
	}

	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // DefaultTransport is *http.Transport.
	transport.MaxIdleConnsPerHost = cfg.idleConns

	client := &http.Client{
		Transport: setupAuth(
			setupRetry(
				setupLimit(transport, cfg.limit),
				cfg.maxRetries,
			),
			apiKey,
			strconv.Itoa(version),
		),
		Timeout: cfg.timeout,
	}

	return &Client{
		http: client,
		host: cfg.host,
	}
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	endpoint, err := url.JoinPath(c.host, path)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if err := json.UnmarshalRead(resp.Body, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}
