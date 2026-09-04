package engine

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/provider"
)

// SyncEngine coordinates reading input files, polling providers, and updating feeds.
type SyncEngine struct {
	baseDir      string
	baseURL      string
	storage      *feed.Storage
	mu           sync.Mutex
	isSyncing    bool
	lastSyncTime time.Time
	lastError    string
}

// NewSyncEngine creates a new SyncEngine.
func NewSyncEngine(baseDir, baseURL string, storage *feed.Storage) *SyncEngine {
	return &SyncEngine{
		baseDir: baseDir,
		baseURL: baseURL,
		storage: storage,
	}
}

// Status returns current sync engine state.
func (e *SyncEngine) Status() (isSyncing bool, lastSync time.Time, lastErr string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.isSyncing, e.lastSyncTime, e.lastError
}

// TriggerSync initiates an asynchronous sync run if not already in progress.
// Returns true if a new sync was started, false if already running.
func (e *SyncEngine) TriggerSync(ctx context.Context) bool {
	e.mu.Lock()
	if e.isSyncing {
		e.mu.Unlock()
		return false
	}
	e.isSyncing = true
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.isSyncing = false
			e.lastSyncTime = time.Now()
			e.mu.Unlock()
		}()

		if err := e.runSync(ctx); err != nil {
			log.Printf("[SyncEngine] Sync cycle completed with errors: %v", err)
			e.mu.Lock()
			e.lastError = err.Error()
			e.mu.Unlock()
		} else {
			log.Printf("[SyncEngine] Sync cycle completed successfully")
			e.mu.Lock()
			e.lastError = ""
			e.mu.Unlock()
		}
	}()

	return true
}

// SyncAll runs a synchronous sync run across all discovered providers.
func (e *SyncEngine) SyncAll(ctx context.Context) error {
	e.mu.Lock()
	if e.isSyncing {
		e.mu.Unlock()
		return fmt.Errorf("sync is already in progress")
	}
	e.isSyncing = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.isSyncing = false
		e.lastSyncTime = time.Now()
		e.mu.Unlock()
	}()

	err := e.runSync(ctx)
	if err != nil {
		e.mu.Lock()
		e.lastError = err.Error()
		e.mu.Unlock()
	} else {
		e.mu.Lock()
		e.lastError = ""
		e.mu.Unlock()
	}
	return err
}

func (e *SyncEngine) runSync(ctx context.Context) error {
	activeProviders := provider.DiscoverActiveProviders(e.baseDir)
	if len(activeProviders) == 0 {
		log.Printf("[SyncEngine] No active platform handle files found in %q (e.g. %s/instagram.txt)", e.baseDir, e.baseDir)
		return nil
	}

	var aggregatedErrors []string

	for _, p := range activeProviders {
		sourceFiles := provider.GetPlatformSourceFiles(e.baseDir, p.Name())
		log.Printf("[SyncEngine] Processing platform %s from %s", p.Name(), strings.Join(sourceFiles, ", "))

		var targets []*model.Profile
		seenHandles := make(map[string]bool)

		for _, sourcePath := range sourceFiles {
			file, err := os.Open(sourcePath)
			if err != nil {
				msg := fmt.Sprintf("opening %s: %v", sourcePath, err)
				log.Printf("[SyncEngine] Warning: %s", msg)
				aggregatedErrors = append(aggregatedErrors, msg)
				continue
			}

			scanner := bufio.NewScanner(file)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				targetProfile, err := p.ParseTarget(line)
				if err != nil {
					log.Printf("[SyncEngine] [%s:%d] Skipping invalid line %q: %v", filepath.Base(sourcePath), lineNum, line, err)
					continue
				}

				normalizedKey := strings.ToLower(targetProfile.Handle)
				if !seenHandles[normalizedKey] {
					seenHandles[normalizedKey] = true
					targets = append(targets, targetProfile)
				}
			}
			file.Close()

			if err := scanner.Err(); err != nil {
				msg := fmt.Sprintf("scanning %s: %v", sourcePath, err)
				log.Printf("[SyncEngine] Warning: %s", msg)
				aggregatedErrors = append(aggregatedErrors, msg)
			}
		}

		for i, targetProfile := range targets {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if i > 0 && p.Name() != "mockplatform" {
				// Polite spacing between profile scrapes to avoid triggering Instagram WAF rate limits
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2500 * time.Millisecond):
				}
			}

			sinceID := e.storage.GetLatestPostID(p.Name(), targetProfile.Handle)
			log.Printf("[SyncEngine] Fetching posts for %s @%s (since post ID: %q)...", p.Name(), targetProfile.Handle, sinceID)

			posts, err := p.FetchPosts(ctx, targetProfile, sinceID)
			if err != nil {
				msg := fmt.Sprintf("[%s @%s] fetch failed: %v", p.Name(), targetProfile.Handle, err)
				log.Printf("[SyncEngine] Warning: %s (preserving existing feed)", msg)
				aggregatedErrors = append(aggregatedErrors, msg)
				continue
			}

			log.Printf("[SyncEngine] Fetched %d new/recent posts for %s @%s", len(posts), p.Name(), targetProfile.Handle)

			if err := e.storage.SavePosts(e.baseURL, targetProfile, posts); err != nil {
				msg := fmt.Sprintf("[%s @%s] failed saving feed: %v", p.Name(), targetProfile.Handle, err)
				log.Printf("[SyncEngine] Error: %s", msg)
				aggregatedErrors = append(aggregatedErrors, msg)
				continue
			}
		}
	}

	if len(aggregatedErrors) > 0 {
		return fmt.Errorf("%d issue(s) encountered during sync: %s", len(aggregatedErrors), strings.Join(aggregatedErrors, "; "))
	}
	return nil
}

// StartScheduler starts background ticker for periodic sync.
func (e *SyncEngine) StartScheduler(ctx context.Context, freq time.Duration) {
	log.Printf("[SyncEngine] Scheduling periodic sync every %s", freq)

	// Run initial sync asynchronously on startup
	e.TriggerSync(ctx)

	ticker := time.NewTicker(freq)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("[SyncEngine] Periodic ticker triggered")
				e.TriggerSync(ctx)
			case <-ctx.Done():
				log.Printf("[SyncEngine] Scheduler stopped")
				return
			}
		}
	}()
}
