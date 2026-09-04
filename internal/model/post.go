package model

import "time"

// Post represents a single social media post's metadata.
type Post struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Caption      string    `json:"caption"`
	ThumbnailURL string    `json:"thumbnail_url"`
	PublishedAt  time.Time `json:"published_at"`
	Author       string    `json:"author"`
	IsVideo      bool      `json:"is_video"`
	IsCollab     bool      `json:"is_collab,omitempty"`
}

// Profile represents a target social media user profile.
type Profile struct {
	Platform string `json:"platform"`
	Handle   string `json:"handle"`
	URL      string `json:"url"`
	FullName string `json:"full_name,omitempty"`
	Bio      string `json:"bio,omitempty"`
}

// FeedStats captures file statistics and item counts for UI display.
type FeedStats struct {
	Platform            string    `json:"platform"`
	Handle              string    `json:"handle"`
	ProfileURL          string    `json:"profile_url"`
	RecentItemCount     int       `json:"recent_item_count"`
	AllItemCount        int       `json:"all_item_count"`
	RecentFileSizeBytes int64     `json:"recent_file_size_bytes"`
	AllFileSizeBytes    int64     `json:"all_file_size_bytes"`
	LastUpdated         time.Time `json:"last_updated"`
	RecentFeedURL       string    `json:"recent_feed_url"`
	AllFeedURL          string    `json:"all_feed_url"`
}
