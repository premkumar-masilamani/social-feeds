package feed

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

const (
	RecentFeedLimit = 50
	RecentSuffix    = "-feed.xml"
	AllSuffix       = "-all-feed.xml"
)

// Storage manages local Atom feed XML files on disk.
type Storage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewStorage creates a new Storage manager pointing to baseDir (e.g. "./feeds").
func NewStorage(baseDir string) *Storage {
	return &Storage{
		baseDir: baseDir,
	}
}

// BaseDir returns the root storage directory path.
func (s *Storage) BaseDir() string {
	return s.baseDir
}

// GetRecentFeedPath returns the file path for the 50-item feed.
func (s *Storage) GetRecentFeedPath(platform, handle string) string {
	return filepath.Join(s.baseDir, platform, handle+RecentSuffix)
}

// GetAllFeedPath returns the file path for the full-archive feed.
func (s *Storage) GetAllFeedPath(platform, handle string) string {
	return filepath.Join(s.baseDir, platform, handle+AllSuffix)
}

// ReadExistingPosts loads all posts currently preserved in the full-archive feed.
func (s *Storage) ReadExistingPosts(platform, handle string) ([]model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	archivePath := s.GetAllFeedPath(platform, handle)
	data, err := os.ReadFile(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading archive feed %q: %w", archivePath, err)
	}

	atomFeed, err := ParseAtomFeed(data)
	if err != nil {
		return nil, fmt.Errorf("parsing existing feed XML %q: %w", archivePath, err)
	}

	posts := make([]model.Post, 0, len(atomFeed.Entries))
	for _, entry := range atomFeed.Entries {
		pubTime, _ := time.Parse(time.RFC3339, entry.Published)
		author := ""
		if entry.Author != nil {
			author = entry.Author.Name
		}
		posts = append(posts, model.Post{
			ID:          entry.ID,
			URL:         entry.Link.Href,
			Caption:     entry.Title,
			PublishedAt: pubTime,
			Author:      author,
		})
	}

	return posts, nil
}

// GetLatestPostID returns the ID of the most recent post stored in the archive feed.
func (s *Storage) GetLatestPostID(platform, handle string) string {
	posts, err := s.ReadExistingPosts(platform, handle)
	if err != nil || len(posts) == 0 {
		return ""
	}
	return posts[0].ID
}

// SavePosts idempotently merges newly fetched posts with existing posts and writes both feeds.
func (s *Storage) SavePosts(baseURL string, profile *model.Profile, newPosts []model.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	platformDir := filepath.Join(s.baseDir, profile.Platform)
	if err := os.MkdirAll(platformDir, 0755); err != nil {
		return fmt.Errorf("creating platform directory %q: %w", platformDir, err)
	}

	archivePath := s.GetAllFeedPath(profile.Platform, profile.Handle)
	existingPosts := make([]model.Post, 0)
	if data, err := os.ReadFile(archivePath); err == nil {
		if atomFeed, err := ParseAtomFeed(data); err == nil {
			for _, entry := range atomFeed.Entries {
				pubTime, _ := time.Parse(time.RFC3339, entry.Published)
				author := ""
				if entry.Author != nil {
					author = entry.Author.Name
				}
				existingPosts = append(existingPosts, model.Post{
					ID:          entry.ID,
					URL:         entry.Link.Href,
					Caption:     entry.Title,
					PublishedAt: pubTime,
					Author:      author,
				})
			}
		}
	}

	// Merge with de-duplication
	seenIDs := make(map[string]bool)
	merged := make([]model.Post, 0, len(newPosts)+len(existingPosts))

	// Add new posts first
	for _, p := range newPosts {
		key := p.URL
		if key == "" {
			key = p.ID
		}
		if !seenIDs[key] {
			seenIDs[key] = true
			merged = append(merged, p)
		}
	}

	// Append existing posts if not already present
	for _, p := range existingPosts {
		key := p.URL
		if key == "" {
			key = p.ID
		}
		if !seenIDs[key] {
			seenIDs[key] = true
			merged = append(merged, p)
		}
	}

	// Sort chronologically descending (newest first)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].PublishedAt.After(merged[j].PublishedAt)
	})

	baseURL = strings.TrimRight(baseURL, "/")

	// 1. Generate and save full-archive feed
	allFeedSelfURL := fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, profile.Platform, profile.Handle, AllSuffix)
	allFeedXML, err := GenerateAtomXML(profile, merged, allFeedSelfURL)
	if err != nil {
		return fmt.Errorf("generating archive atom XML: %w", err)
	}
	if err := os.WriteFile(archivePath, allFeedXML, 0644); err != nil {
		return fmt.Errorf("writing archive feed to %q: %w", archivePath, err)
	}

	// 2. Generate and save capped recent feed (latest 50)
	recentPosts := merged
	if len(recentPosts) > RecentFeedLimit {
		recentPosts = recentPosts[:RecentFeedLimit]
	}

	recentFeedSelfURL := fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, profile.Platform, profile.Handle, RecentSuffix)
	recentFeedXML, err := GenerateAtomXML(profile, recentPosts, recentFeedSelfURL)
	if err != nil {
		return fmt.Errorf("generating recent atom XML: %w", err)
	}
	recentPath := s.GetRecentFeedPath(profile.Platform, profile.Handle)
	if err := os.WriteFile(recentPath, recentFeedXML, 0644); err != nil {
		return fmt.Errorf("writing recent feed to %q: %w", recentPath, err)
	}

	return nil
}

// GetFeedStats collects statistics for all generated feeds under baseDir.
func (s *Storage) GetFeedStats(baseURL string) ([]model.FeedStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	baseURL = strings.TrimRight(baseURL, "/")
	var statsList []model.FeedStats

	platformEntries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return statsList, nil
		}
		return nil, fmt.Errorf("reading base directory %q: %w", s.baseDir, err)
	}

	for _, pEntry := range platformEntries {
		if !pEntry.IsDir() {
			continue
		}
		platform := pEntry.Name()
		platformPath := filepath.Join(s.baseDir, platform)
		fileEntries, err := os.ReadDir(platformPath)
		if err != nil {
			continue
		}

		// Map handles to their feed file information
		handles := make(map[string]bool)
		for _, f := range fileEntries {
			name := f.Name()
			if strings.HasSuffix(name, AllSuffix) {
				handle := strings.TrimSuffix(name, AllSuffix)
				handles[handle] = true
			} else if strings.HasSuffix(name, RecentSuffix) {
				handle := strings.TrimSuffix(name, RecentSuffix)
				handles[handle] = true
			}
		}

		for handle := range handles {
			recentPath := s.GetRecentFeedPath(platform, handle)
			allPath := s.GetAllFeedPath(platform, handle)

			var recentSize, allSize int64
			var recentCount, allCount int
			var lastUpdated time.Time

			if fi, err := os.Stat(recentPath); err == nil {
				recentSize = fi.Size()
				if fi.ModTime().After(lastUpdated) {
					lastUpdated = fi.ModTime()
				}
				if data, err := os.ReadFile(recentPath); err == nil {
					if feed, err := ParseAtomFeed(data); err == nil {
						recentCount = len(feed.Entries)
					}
				}
			}

			if fi, err := os.Stat(allPath); err == nil {
				allSize = fi.Size()
				if fi.ModTime().After(lastUpdated) {
					lastUpdated = fi.ModTime()
				}
				if data, err := os.ReadFile(allPath); err == nil {
					if feed, err := ParseAtomFeed(data); err == nil {
						allCount = len(feed.Entries)
					}
				}
			}

			profileURL := fmt.Sprintf("https://www.instagram.com/%s/", handle)
			if platform == "x" {
				profileURL = fmt.Sprintf("https://x.com/%s", handle)
			} else if platform == "facebook" {
				profileURL = fmt.Sprintf("https://facebook.com/%s", handle)
			}

			statsList = append(statsList, model.FeedStats{
				Platform:            platform,
				Handle:              handle,
				ProfileURL:          profileURL,
				RecentItemCount:     recentCount,
				AllItemCount:        allCount,
				RecentFileSizeBytes: recentSize,
				AllFileSizeBytes:    allSize,
				LastUpdated:         lastUpdated,
				RecentFeedURL:       fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, platform, handle, RecentSuffix),
				AllFeedURL:          fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, platform, handle, AllSuffix),
			})
		}
	}

	sort.Slice(statsList, func(i, j int) bool {
		if statsList[i].Platform != statsList[j].Platform {
			return statsList[i].Platform < statsList[j].Platform
		}
		return statsList[i].Handle < statsList[j].Handle
	})

	return statsList, nil
}
