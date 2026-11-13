// Package config loads the service runtime configuration from the environment.
//
// Configuration is read once at start-up, validated eagerly and then treated as
// immutable. Invalid values fail fast so that a misconfigured task never reports
// healthy and receives traffic.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	defaultServiceName     = "items-api"
	defaultEnvironment     = "local"
	defaultHTTPPort        = 8080
	defaultMetricsPort     = 9090
	defaultMaxPageSize     = 100
	defaultShutdownTimeout = 20 * time.Second
	defaultReadyDrainDelay = 5 * time.Second

	maxPort            = 65535
	minAPITokenLength  = 16
	maxShutdownTimeout = 60 * time.Second
	maxReadyDrainDelay = 30 * time.Second
	maxPageSizeLimit   = 1000
)

// Config is the validated, immutable runtime configuration.
type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	HTTPPort        int
	MetricsPort     int
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	ReadyDrainDelay time.Duration
	MaxPageSize     int
	// APIToken protects write endpoints. It is injected from Secrets Manager in
	// deployed environments and is mandatory in prod.
	APIToken string
}

// Lookup matches the signature of os.LookupEnv so tests can supply a map.
type Lookup func(key string) (string, bool)

// Load reads and validates the configuration. All validation problems are
// reported together so an operator can fix them in one pass.
func Load(lookup Lookup) (Config, error) {
	p := &parser{lookup: lookup}
	c := Config{
		ServiceName:     p.str("SERVICE_NAME", defaultServiceName),
		ServiceVersion:  p.str("SERVICE_VERSION", "dev"),
		Environment:     p.str("APP_ENV", defaultEnvironment),
		HTTPPort:        p.integer("HTTP_PORT", defaultHTTPPort, 1, maxPort),
		MetricsPort:     p.integer("METRICS_PORT", defaultMetricsPort, 1, maxPort),
		LogLevel:        p.level("LOG_LEVEL"),
		ShutdownTimeout: p.duration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout, maxShutdownTimeout),
		ReadyDrainDelay: p.duration("READY_DRAIN_DELAY", defaultReadyDrainDelay, maxReadyDrainDelay),
		MaxPageSize:     p.integer("MAX_PAGE_SIZE", defaultMaxPageSize, 1, maxPageSizeLimit),
		APIToken:        p.str("API_TOKEN", ""),
	}
	c.validate(p)
	if len(p.errs) > 0 {
		return Config{}, errors.Join(p.errs...)
	}
	return c, nil
}

func (c Config) validate(p *parser) {
	if c.HTTPPort == c.MetricsPort {
		p.errs = append(p.errs, fmt.Errorf("HTTP_PORT and METRICS_PORT must differ, both are %d", c.HTTPPort))
	}
	if c.APIToken != "" && len(c.APIToken) < minAPITokenLength {
		p.errs = append(p.errs, fmt.Errorf("API_TOKEN must be at least %d characters", minAPITokenLength))
	}
	if c.Environment == "prod" && c.APIToken == "" {
		p.errs = append(p.errs, errors.New("API_TOKEN is required when APP_ENV=prod"))
	}
}

// parser accumulates validation errors while keeping call sites linear.
type parser struct {
	lookup Lookup
	errs   []error
}

func (p *parser) raw(key string) (string, bool) {
	v, ok := p.lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (p *parser) str(key, def string) string {
	if v, ok := p.raw(key); ok {
		return v
	}
	return def
}

func (p *parser) integer(key string, def, lo, hi int) int {
	v, ok := p.raw(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		p.errs = append(p.errs, fmt.Errorf("%s must be an integer in [%d, %d], got %q", key, lo, hi, v))
		return def
	}
	return n
}

func (p *parser) duration(key string, def, hi time.Duration) time.Duration {
	v, ok := p.raw(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 || d > hi {
		p.errs = append(p.errs, fmt.Errorf("%s must be a duration in [0s, %s], got %q", key, hi, v))
		return def
	}
	return d
}

func (p *parser) level(key string) slog.Level {
	v, ok := p.raw(key)
	if !ok {
		return slog.LevelInfo
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToLower(v))); err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s must be one of debug|info|warn|error, got %q", key, v))
		return slog.LevelInfo
	}
	return lvl
}
