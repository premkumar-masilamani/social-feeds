package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/config"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/engine"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
	_ "github.com/premkumar-masilamani/social-media-rss-feed/internal/provider/instagram"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/server"
)

// ============================================================================
// CONFIGURABLE CONSTANTS (As requested in requirements)
// ============================================================================

// DefaultPollFrequency sets how often the background scheduler checks for new content.
// Adjust this constant as desired (e.g. 5*time.Minute, 1*time.Hour, 24*time.Hour).
const DefaultPollFrequency = 1 * time.Hour

// DefaultPort sets the default HTTP port.
// Set to 9527 (outside standard software dev ports like 3000, 8080, 5000, 8000)
// so the CLI can run quietly in the background on your machine.
const DefaultPort = 9527

func main() {
	cfg := config.Load(DefaultPort, DefaultPollFrequency)

	log.Printf("=========================================================")
	log.Printf("  Social Media RSS Feed Generator")
	log.Printf("  Port:          %d", cfg.Port)
	log.Printf("  Poll Interval: %s", cfg.PollInterval)
	log.Printf("  Feeds Dir:     %s", cfg.FeedsDir)
	log.Printf("  Handles Dir:   %s", cfg.HandlesDir)
	log.Printf("  Base URL:      %s", cfg.BaseURL)
	log.Printf("=========================================================")

	// Ensure sample handles folder and instagram.txt exists if missing
	ensureSampleInputFiles(cfg.HandlesDir)

	storage := feed.NewStorage(cfg.FeedsDir)
	syncEng := engine.NewSyncEngine(cfg.HandlesDir, cfg.BaseURL, storage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background delta scheduler
	syncEng.StartScheduler(ctx, cfg.PollInterval)

	// Start HTTP Web Server
	srv := server.NewServer(cfg.Port, cfg.BaseURL, syncEng, storage)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	// Listen for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("Server startup failed: %v", err)
	case sig := <-sigCh:
		log.Printf("Received signal %s, shutting down gracefully...", sig)
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	log.Println("Social Media RSS Feed service stopped.")
}

func ensureSampleInputFiles(handlesDir string) {
	if err := os.MkdirAll(handlesDir, 0755); err != nil {
		log.Printf("[Warning] Failed creating handles directory %q: %v", handlesDir, err)
		return
	}

	sampleFile := filepath.Join(handlesDir, "instagram.txt")
	if _, err := os.Stat(sampleFile); os.IsNotExist(err) {
		content := `# Instagram Handles (one handle per line)
#
# Format:
#   username
#   @username
#
# How to extract the handle name from a URL:
#   - From Profile URL (https://www.instagram.com/myu_stories/):
#     Take the segment after 'instagram.com/': "myu_stories"
#   - From Share Links (https://www.instagram.com/natgeo/?igsh=...):
#     Ignore '?igsh=...' and take the username segment: "natgeo"
#   - From Post / Reel URLs (https://www.instagram.com/reel/DcFuLeATEc9/):
#     On the reel or photo page, look at the creator name/avatar and use that handle.
#
# Private Handles:
#   To track handles locally without committing them to git, create:
#   handles/instagram.local.txt (or place in handles/private/instagram.txt)

myu_stories
`
		if err := os.WriteFile(sampleFile, []byte(content), 0644); err == nil {
			fmt.Printf("Created starter %s file with sample handles.\n", sampleFile)
		}
	}
}
