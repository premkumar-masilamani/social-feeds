package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("FEEDS_DIR")
	os.Unsetenv("BASE_URL")

	cfg := LoadFromArgs(nil, 9527, 1*time.Hour)
	if cfg.Port != 9527 {
		t.Errorf("expected default port 9527, got %d", cfg.Port)
	}
	if cfg.PollInterval != 1*time.Hour {
		t.Errorf("expected default poll interval 1h, got %v", cfg.PollInterval)
	}
	if cfg.FeedsDir != "./feeds" {
		t.Errorf("expected default feeds dir ./feeds, got %q", cfg.FeedsDir)
	}
	if cfg.BaseURL != "http://localhost:9527" {
		t.Errorf("expected default base url http://localhost:9527, got %q", cfg.BaseURL)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "8888")
	t.Setenv("FEEDS_DIR", "/tmp/custom_feeds")
	t.Setenv("BASE_URL", "https://rss.example.com")

	cfg := LoadFromArgs(nil, 9527, 30*time.Minute)
	if cfg.Port != 8888 {
		t.Errorf("expected overridden port 8888, got %d", cfg.Port)
	}
	if cfg.FeedsDir != "/tmp/custom_feeds" {
		t.Errorf("expected overridden feeds dir, got %q", cfg.FeedsDir)
	}
	if cfg.BaseURL != "https://rss.example.com" {
		t.Errorf("expected overridden base url, got %q", cfg.BaseURL)
	}
}
