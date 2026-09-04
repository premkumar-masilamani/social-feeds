package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime options.
type Config struct {
	Port         int
	PollInterval time.Duration
	FeedsDir     string
	BaseURL      string
}

// Load loads configuration from flags and environment variables.
func Load(defaultPort int, defaultPollInterval time.Duration) *Config {
	port := defaultPort
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		}
	}

	feedsDir := os.Getenv("FEEDS_DIR")
	if feedsDir == "" {
		feedsDir = "./feeds"
	}

	baseURL := os.Getenv("BASE_URL")

	flagPort := flag.Int("port", port, "HTTP server listening port")
	flagPoll := flag.Duration("poll", defaultPollInterval, "Periodic poll interval (e.g. 5m, 1h, 24h)")
	flagDir := flag.String("feeds-dir", feedsDir, "Directory to store generated feeds")
	flagURL := flag.String("base-url", baseURL, "Base URL for feed links in XML (defaults to http://localhost:<port>)")
	flag.Parse()

	finalURL := *flagURL
	if finalURL == "" {
		finalURL = fmt.Sprintf("http://localhost:%d", *flagPort)
	}

	return &Config{
		Port:         *flagPort,
		PollInterval: *flagPoll,
		FeedsDir:     *flagDir,
		BaseURL:      finalURL,
	}
}
