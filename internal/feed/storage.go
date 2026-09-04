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
	FeedLimit  = 25
	FeedSuffix = ".xml"

	// Backward compatibility aliases
	RecentFeedLimit = FeedLimit
	RecentSuffix    = FeedSuffix
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

// GetFeedPath returns the file path for the handle's feed XML (capped at FeedLimit).
func (s *Storage) GetFeedPath(platform, handle string) string {
	return filepath.Join(s.baseDir, platform, handle+FeedSuffix)
}

// GetRecentFeedPath is an alias for GetFeedPath for backward compatibility.
func (s *Storage) GetRecentFeedPath(platform, handle string) string {
	return s.GetFeedPath(platform, handle)
}

// ReadExistingPosts loads all posts currently preserved in the handle's feed.
func (s *Storage) ReadExistingPosts(platform, handle string) ([]model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feedPath := s.GetFeedPath(platform, handle)
	data, err := os.ReadFile(feedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback: check legacy -feed.xml if not yet renamed
			legacyPath := filepath.Join(s.baseDir, platform, handle+"-feed.xml")
			if legacyData, lErr := os.ReadFile(legacyPath); lErr == nil {
				data = legacyData
			} else {
				return nil, nil
			}
		} else {
			return nil, fmt.Errorf("reading feed %q: %w", feedPath, err)
		}
	}

	atomFeed, err := ParseAtomFeed(data)
	if err != nil {
		return nil, fmt.Errorf("parsing existing feed XML %q: %w", feedPath, err)
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

// GetLatestPostID returns the ID of the most recent post stored in the feed.
func (s *Storage) GetLatestPostID(platform, handle string) string {
	posts, err := s.ReadExistingPosts(platform, handle)
	if err != nil || len(posts) == 0 {
		return ""
	}
	return posts[0].ID
}

// SavePosts idempotently merges newly fetched posts with existing posts and writes the feed capped at FeedLimit.
func (s *Storage) SavePosts(baseURL string, profile *model.Profile, newPosts []model.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	platformDir := filepath.Join(s.baseDir, profile.Platform)
	if err := os.MkdirAll(platformDir, 0755); err != nil {
		return fmt.Errorf("creating platform directory %q: %w", platformDir, err)
	}

	feedPath := s.GetFeedPath(profile.Platform, profile.Handle)
	existingPosts := make([]model.Post, 0)
	readPath := feedPath
	if _, err := os.Stat(readPath); os.IsNotExist(err) {
		legacyPath := filepath.Join(s.baseDir, profile.Platform, profile.Handle+"-feed.xml")
		if _, err := os.Stat(legacyPath); err == nil {
			readPath = legacyPath
		}
	}
	if data, err := os.ReadFile(readPath); err == nil {
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

	// Cap at FeedLimit (25 posts)
	if len(merged) > FeedLimit {
		merged = merged[:FeedLimit]
	}

	baseURL = strings.TrimRight(baseURL, "/")

	feedSelfURL := fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, profile.Platform, profile.Handle, FeedSuffix)
	feedXML, err := GenerateAtomXML(profile, merged, feedSelfURL)
	if err != nil {
		return fmt.Errorf("generating atom XML: %w", err)
	}
	if err := os.WriteFile(feedPath, feedXML, 0644); err != nil {
		return fmt.Errorf("writing feed to %q: %w", feedPath, err)
	}

	// Clean up legacy -feed.xml and -all-feed.xml if they existed
	legacyFeedPath := filepath.Join(s.baseDir, profile.Platform, profile.Handle+"-feed.xml")
	if legacyFeedPath != feedPath {
		_ = os.Remove(legacyFeedPath)
	}
	legacyAllPath := filepath.Join(s.baseDir, profile.Platform, profile.Handle+"-all-feed.xml")
	_ = os.Remove(legacyAllPath)

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
			if strings.HasSuffix(name, FeedSuffix) {
				handle := strings.TrimSuffix(name, FeedSuffix)
				handle = strings.TrimSuffix(handle, "-feed")
				handle = strings.TrimSuffix(handle, "-all-feed")
				handles[handle] = true
			}
		}

		for handle := range handles {
			feedPath := s.GetFeedPath(platform, handle)

			var fileSize int64
			var itemCount int
			var lastUpdated time.Time

			if fi, err := os.Stat(feedPath); err == nil {
				fileSize = fi.Size()
				lastUpdated = fi.ModTime()
				if data, err := os.ReadFile(feedPath); err == nil {
					if feed, err := ParseAtomFeed(data); err == nil {
						itemCount = len(feed.Entries)
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
				Platform:      platform,
				Handle:        handle,
				ProfileURL:    profileURL,
				ItemCount:     itemCount,
				FileSizeBytes: fileSize,
				LastUpdated:   lastUpdated,
				FeedURL:       fmt.Sprintf("%s/feeds/%s/%s%s", baseURL, platform, handle, FeedSuffix),
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
