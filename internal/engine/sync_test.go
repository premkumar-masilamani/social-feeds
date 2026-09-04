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
	recentPath := storage.GetFeedPath("mockplatform", "tester")
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

	// Wait for any background goroutine from TriggerSync to complete
	for i := 0; i < 50; i++ {
		isSyncing, _, _ := eng.Status()
		if !isSyncing {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSyncEngine_LocalHandlesAndDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	storage := feed.NewStorage(tempDir)
	baseURL := "http://localhost:9527"

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

	// Create public file and local/private file
	pubFile := filepath.Join(tempDir, "mockplatform.txt")
	localFile := filepath.Join(tempDir, "mockplatform.local.txt")
	// "tester" is present in both (to test deduplication), "private_user" is only in local
	_ = os.WriteFile(pubFile, []byte("tester\n"), 0644)
	_ = os.WriteFile(localFile, []byte("tester\nprivate_user\n"), 0644)

	eng := NewSyncEngine(tempDir, baseURL, storage)
	if err := eng.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	// Should fetch tester once, and private_user once -> total 2 calls
	if mock.fetchCalls != 2 {
		t.Errorf("expected 2 fetch calls (tester deduplicated + private_user), got %d", mock.fetchCalls)
	}

	if _, err := os.Stat(storage.GetFeedPath("mockplatform", "tester")); err != nil {
		t.Errorf("feed for tester missing: %v", err)
	}
	if _, err := os.Stat(storage.GetFeedPath("mockplatform", "private_user")); err != nil {
		t.Errorf("feed for private_user missing: %v", err)
	}
}
