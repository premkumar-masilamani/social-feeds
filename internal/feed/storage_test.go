package feed

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

func TestSavePostsAndIdempotentDelta(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewStorage(tempDir)

	profile := &model.Profile{
		Platform: "instagram",
		Handle:   "testuser",
		URL:      "https://www.instagram.com/testuser/",
	}

	initialPosts := []model.Post{
		{
			ID:          "p1",
			URL:         "https://www.instagram.com/p/p1/",
			Caption:     "Post 1",
			PublishedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
			Author:      "testuser",
		},
		{
			ID:          "p2",
			URL:         "https://www.instagram.com/p/p2/",
			Caption:     "Post 2",
			PublishedAt: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
			Author:      "testuser",
		},
	}

	baseURL := "http://localhost:9527"

	// First save
	if err := storage.SavePosts(baseURL, profile, initialPosts); err != nil {
		t.Fatalf("unexpected error on initial save: %v", err)
	}

	feedPath := storage.GetFeedPath("instagram", "testuser")
	if _, err := os.Stat(feedPath); err != nil {
		t.Fatalf("expected feed at %q, but missing: %v", feedPath, err)
	}

	// Verify latest post ID
	latestID := storage.GetLatestPostID("instagram", "testuser")
	if latestID != "p2" && latestID != "https://www.instagram.com/p/p2/" {
		t.Errorf("got latest post ID %q, expected p2 or its URL", latestID)
	}

	// Delta save: include 1 new post ("p3") and 1 existing post ("p2")
	deltaPosts := []model.Post{
		{
			ID:          "p3",
			URL:         "https://www.instagram.com/p/p3/",
			Caption:     "Post 3",
			PublishedAt: time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
			Author:      "testuser",
		},
		{
			ID:          "p2",
			URL:         "https://www.instagram.com/p/p2/",
			Caption:     "Post 2 Updated",
			PublishedAt: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
			Author:      "testuser",
		},
	}

	if err := storage.SavePosts(baseURL, profile, deltaPosts); err != nil {
		t.Fatalf("unexpected error on delta save: %v", err)
	}

	// Read and verify feed has exactly 3 unique posts (p3, p2, p1)
	data, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("failed reading feed: %v", err)
	}

	atomFeed, err := ParseAtomFeed(data)
	if err != nil {
		t.Fatalf("failed parsing feed XML: %v", err)
	}

	if len(atomFeed.Entries) != 3 {
		t.Fatalf("expected 3 entries in merged feed, got %d", len(atomFeed.Entries))
	}

	// Verify newest post is at the top
	if atomFeed.Entries[0].ID != "https://www.instagram.com/p/p3/" {
		t.Errorf("expected newest entry to be p3, got %s", atomFeed.Entries[0].ID)
	}
}

func TestCapFeedAtLimit(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewStorage(tempDir)

	profile := &model.Profile{
		Platform: "instagram",
		Handle:   "activeuser",
		URL:      "https://www.instagram.com/activeuser/",
	}

	// Generate 65 posts
	posts := make([]model.Post, 65)
	for i := 0; i < 65; i++ {
		posts[i] = model.Post{
			ID:          fmt.Sprintf("item-%03d", i),
			URL:         fmt.Sprintf("https://www.instagram.com/p/item-%03d/", i),
			Caption:     fmt.Sprintf("Post number %d", i),
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour),
			Author:      "activeuser",
		}
	}

	baseURL := "http://localhost:9527"
	if err := storage.SavePosts(baseURL, profile, posts); err != nil {
		t.Fatalf("failed saving 65 posts: %v", err)
	}

	feedPath := storage.GetFeedPath("instagram", "activeuser")

	feedData, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("failed reading feed: %v", err)
	}
	atomFeed, err := ParseAtomFeed(feedData)
	if err != nil {
		t.Fatalf("failed parsing feed: %v", err)
	}
	if len(atomFeed.Entries) != FeedLimit {
		t.Errorf("expected exactly %d entries in feed, got %d", FeedLimit, len(atomFeed.Entries))
	}

	// Test GetFeedStats
	stats, err := storage.GetFeedStats(baseURL)
	if err != nil {
		t.Fatalf("unexpected error getting feed stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat entry, got %d", len(stats))
	}
	s := stats[0]
	if s.ItemCount != FeedLimit {
		t.Errorf("expected item count %d, got %d", FeedLimit, s.ItemCount)
	}
	if s.FileSizeBytes <= 0 {
		t.Errorf("expected positive file size, got %d", s.FileSizeBytes)
	}
	expectedFeedURL := fmt.Sprintf("%s/feeds/instagram/activeuser-feed.xml", baseURL)
	if s.FeedURL != expectedFeedURL {
		t.Errorf("expected FeedURL %q, got %q", expectedFeedURL, s.FeedURL)
	}
}

func TestDirectoryIsolation(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewStorage(tempDir)

	expectedFeed := filepath.Join(tempDir, "instagram", "handle-feed.xml")

	if storage.GetFeedPath("instagram", "handle") != expectedFeed {
		t.Errorf("unexpected feed path: got %s, want %s", storage.GetFeedPath("instagram", "handle"), expectedFeed)
	}
}
