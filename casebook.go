package casebook

import (
	"context"
	"fmt"
	"io"
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

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, path, http.MethodGet, nil)
}

func (c *Client) Post(ctx context.Context, path string, buf io.Reader) ([]byte, error) {
	return c.do(ctx, path, http.MethodPost, buf)
}

func (c *Client) do(ctx context.Context, path, method string, buf io.Reader) ([]byte, error) {
	ref, err := url.Parse(c.host + path)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, ref.String(), buf)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return body, nil
}
