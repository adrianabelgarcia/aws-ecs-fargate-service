package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func envOf(m map[string]string) Lookup {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()
	cfg, err := Load(envOf(nil))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8080 || cfg.MetricsPort != 9090 {
		t.Errorf("ports = %d/%d, want 8080/9090", cfg.HTTPPort, cfg.MetricsPort)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.ShutdownTimeout != 20*time.Second || cfg.ReadyDrainDelay != 5*time.Second {
		t.Errorf("timeouts = %s/%s, want 20s/5s", cfg.ShutdownTimeout, cfg.ReadyDrainDelay)
	}
	if cfg.MaxPageSize != 100 || cfg.ServiceVersion != "dev" || cfg.Environment != "local" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Parallel()
	cfg, err := Load(envOf(map[string]string{
		"SERVICE_NAME":      " orders ",
		"SERVICE_VERSION":   "1.2.3",
		"APP_ENV":           "staging",
		"HTTP_PORT":         "3000",
		"METRICS_PORT":      "3001",
		"LOG_LEVEL":         "DEBUG",
		"SHUTDOWN_TIMEOUT":  "30s",
		"READY_DRAIN_DELAY": "0s",
		"MAX_PAGE_SIZE":     "50",
		"API_TOKEN":         "0123456789abcdef",
	}))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	want := Config{
		ServiceName: "orders", ServiceVersion: "1.2.3", Environment: "staging",
		HTTPPort: 3000, MetricsPort: 3001, LogLevel: slog.LevelDebug,
		ShutdownTimeout: 30 * time.Second, ReadyDrainDelay: 0, MaxPageSize: 50,
		APIToken: "0123456789abcdef", // gitleaks:allow (test fixture, not a secret)
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"non-numeric port", map[string]string{"HTTP_PORT": "abc"}, "HTTP_PORT must be an integer"},
		{"port out of range", map[string]string{"HTTP_PORT": "70000"}, "HTTP_PORT must be an integer"},
		{"same ports", map[string]string{"HTTP_PORT": "9000", "METRICS_PORT": "9000"}, "must differ"},
		{"bad duration", map[string]string{"SHUTDOWN_TIMEOUT": "soon"}, "SHUTDOWN_TIMEOUT must be a duration"},
		{"duration too large", map[string]string{"READY_DRAIN_DELAY": "1h"}, "READY_DRAIN_DELAY must be a duration"},
		{"bad level", map[string]string{"LOG_LEVEL": "verbose"}, "LOG_LEVEL must be one of"},
		{"page size zero", map[string]string{"MAX_PAGE_SIZE": "0"}, "MAX_PAGE_SIZE must be an integer"},
		{"short token", map[string]string{"API_TOKEN": "short"}, "API_TOKEN must be at least"},
		{"prod without token", map[string]string{"APP_ENV": "prod"}, "API_TOKEN is required when APP_ENV=prod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(envOf(tt.env))
			if err == nil {
				t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_ReportsAllErrorsTogether(t *testing.T) {
	t.Parallel()
	_, err := Load(envOf(map[string]string{"HTTP_PORT": "x", "LOG_LEVEL": "x"}))
	if err == nil || !strings.Contains(err.Error(), "HTTP_PORT") || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("Load() error = %v, want both HTTP_PORT and LOG_LEVEL reported", err)
	}
}

func TestLoad_BlankValuesFallBackToDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := Load(envOf(map[string]string{"HTTP_PORT": "   ", "SERVICE_NAME": ""}))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8080 || cfg.ServiceName != "items-api" {
		t.Errorf("blank values should use defaults, got %+v", cfg)
	}
}
