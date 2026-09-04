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

	recentPath := storage.GetRecentFeedPath("instagram", "testuser")
	allPath := storage.GetAllFeedPath("instagram", "testuser")

	if _, err := os.Stat(recentPath); err != nil {
		t.Fatalf("expected recent feed at %q, but missing: %v", recentPath, err)
	}
	if _, err := os.Stat(allPath); err != nil {
		t.Fatalf("expected all feed at %q, but missing: %v", allPath, err)
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

	// Read and verify archive feed has exactly 3 unique posts (p3, p2, p1)
	allData, err := os.ReadFile(allPath)
	if err != nil {
		t.Fatalf("failed reading all feed: %v", err)
	}

	feed, err := ParseAtomFeed(allData)
	if err != nil {
		t.Fatalf("failed parsing all feed XML: %v", err)
	}

	if len(feed.Entries) != 3 {
		t.Fatalf("expected 3 entries in merged feed, got %d", len(feed.Entries))
	}

	// Verify newest post is at the top
	if feed.Entries[0].ID != "https://www.instagram.com/p/p3/" {
		t.Errorf("expected newest entry to be p3, got %s", feed.Entries[0].ID)
	}
}

func TestCapRecentFeedAt50(t *testing.T) {
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

	recentPath := storage.GetRecentFeedPath("instagram", "activeuser")
	allPath := storage.GetAllFeedPath("instagram", "activeuser")

	recentData, _ := os.ReadFile(recentPath)
	recentFeed, err := ParseAtomFeed(recentData)
	if err != nil {
		t.Fatalf("failed parsing recent feed: %v", err)
	}
	if len(recentFeed.Entries) != RecentFeedLimit {
		t.Errorf("expected exactly %d entries in recent feed, got %d", RecentFeedLimit, len(recentFeed.Entries))
	}

	allData, _ := os.ReadFile(allPath)
	allFeed, err := ParseAtomFeed(allData)
	if err != nil {
		t.Fatalf("failed parsing all feed: %v", err)
	}
	if len(allFeed.Entries) != 65 {
		t.Errorf("expected 65 entries in all feed, got %d", len(allFeed.Entries))
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
	if s.RecentItemCount != 50 || s.AllItemCount != 65 {
		t.Errorf("expected (recent=50, all=65), got (recent=%d, all=%d)", s.RecentItemCount, s.AllItemCount)
	}
	if s.RecentFileSizeBytes <= 0 || s.AllFileSizeBytes <= 0 {
		t.Errorf("expected positive file sizes, got recent=%d, all=%d", s.RecentFileSizeBytes, s.AllFileSizeBytes)
	}
}

func TestDirectoryIsolation(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewStorage(tempDir)

	expectedRecent := filepath.Join(tempDir, "instagram", "handle-feed.xml")
	expectedAll := filepath.Join(tempDir, "instagram", "handle-all-feed.xml")

	if storage.GetRecentFeedPath("instagram", "handle") != expectedRecent {
		t.Errorf("unexpected recent path")
	}
	if storage.GetAllFeedPath("instagram", "handle") != expectedAll {
		t.Errorf("unexpected all path")
	}
}
