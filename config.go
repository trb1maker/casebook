package casebook

import "time"

const (
	defaultHost       = "https://api.casebook.ru"
	defaultMaxRetries = 3
	defaultLimit      = 100
	defaultTimeout    = 10 * time.Second
	defaultIdleConns  = 10
)

type config struct {
	host       string
	maxRetries int
	limit      int
	timeout    time.Duration
	idleConns  int
}

type Option func(*config)

func WithHost(host string) Option {
	return func(c *config) {
		c.host = host
	}
}

func WithMaxRetries(max int) Option {
	return func(c *config) {
		c.maxRetries = max
	}
}

func WithLimit(limit int) Option {
	return func(c *config) {
		c.limit = limit
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

func WithIdleConns(idleConns int) Option {
	return func(c *config) {
		c.idleConns = idleConns
	}
}

func defaultConfig() *config {
	return &config{
		host:       defaultHost,
		maxRetries: defaultMaxRetries,
		limit:      defaultLimit,
		timeout:    defaultTimeout,
		idleConns:  defaultIdleConns,
	}
}
