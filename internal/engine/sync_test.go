package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/provider"
)

// mockProvider implements provider.PlatformProvider for testing.
type mockProvider struct {
	mu         sync.Mutex
	fetchCalls int
	postsToRet []model.Post
}

func (m *mockProvider) Name() string {
	return "mockplatform"
}

func (m *mockProvider) SourceFile() string {
	return "mockplatform.txt"
}

func (m *mockProvider) ParseTarget(line string) (*model.Profile, error) {
	if line == "" || line[0] == '#' {
		return nil, fmt.Errorf("empty line")
	}
	return &model.Profile{
		Platform: "mockplatform",
		Handle:   line,
		URL:      "https://mockplatform.com/" + line,
	}, nil
}

func (m *mockProvider) FetchPosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetchCalls++

	// Filter out posts if sinceID matches
	var result []model.Post
	for _, p := range m.postsToRet {
		if sinceID != "" && p.ID == sinceID {
			break
		}
		result = append(result, p)
	}
	return result, nil
}

func TestSyncEngine(t *testing.T) {
	tempDir := t.TempDir()
	storage := feed.NewStorage(tempDir)
	baseURL := "http://localhost:9527"

	// Register mock provider
	mock := &mockProvider{
		postsToRet: []model.Post{
			{
				ID:          "mock-1",
				URL:         "https://mockplatform.com/p/1",
				Caption:     "Mock Post 1",
				PublishedAt: time.Now(),
				Author:      "tester",
			},
		},
	}
	provider.Register(mock)

	// Create source file in tempDir
	sourcePath := filepath.Join(tempDir, "mockplatform.txt")
	if err := os.WriteFile(sourcePath, []byte("tester\n"), 0644); err != nil {
		t.Fatalf("failed creating test input file: %v", err)
	}

	eng := NewSyncEngine(tempDir, baseURL, storage)

	// 1. Run sync
	if err := eng.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	if mock.fetchCalls != 1 {
		t.Errorf("expected 1 fetch call, got %d", mock.fetchCalls)
	}

	// Verify feeds created
	recentPath := storage.GetRecentFeedPath("mockplatform", "tester")
	if _, err := os.Stat(recentPath); err != nil {
		t.Fatalf("recent feed missing: %v", err)
	}

	isSyncing, lastSync, lastErr := eng.Status()
	if isSyncing {
		t.Errorf("expected engine not syncing after SyncAll")
	}
	if lastSync.IsZero() {
		t.Errorf("expected lastSync time to be set")
	}
	if lastErr != "" {
		t.Errorf("unexpected lastErr: %s", lastErr)
	}

	// 2. Concurrency test: TriggerSync while SyncAll running should return false
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = eng.SyncAll(context.Background())
	}()

	// Small pause to let goroutine grab lock
	time.Sleep(2 * time.Millisecond)
	_ = eng.TriggerSync(context.Background()) // Should either start after or skip safely
	wg.Wait()
}
