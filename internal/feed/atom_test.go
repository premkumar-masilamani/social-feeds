package feed

import (
	"strings"
	"testing"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

func TestGenerateAtomXML(t *testing.T) {
	profile := &model.Profile{
		Platform: "instagram",
		Handle:   "natgeo",
		URL:      "https://www.instagram.com/natgeo/",
		FullName: "National Geographic",
		Bio:      "Inspiring people to care about the planet.",
	}

	posts := []model.Post{
		{
			ID:           "post-123",
			URL:          "https://www.instagram.com/p/post-123/",
			Caption:      "A breathtaking view of the mountains.\n\nPhoto by John Doe.",
			ThumbnailURL: "https://scontent.cdninstagram.com/v/sample.jpg",
			PublishedAt:  time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
			Author:       "natgeo",
			IsVideo:      false,
		},
		{
			ID:           "post-124",
			URL:          "https://www.instagram.com/p/post-124/",
			Caption:      "Underwater marine life.",
			ThumbnailURL: "https://scontent.cdninstagram.com/v/sample2.jpg",
			PublishedAt:  time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
			Author:       "natgeo",
			IsVideo:      true,
		},
	}

	xmlData, err := GenerateAtomXML(profile, posts, "http://localhost:9527/feeds/instagram/natgeo-feed.xml")
	if err != nil {
		t.Fatalf("unexpected error generating Atom XML: %v", err)
	}

	xmlStr := string(xmlData)

	// Check root and required Atom elements
	if !strings.Contains(xmlStr, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("missing standard XML declaration")
	}
	if !strings.Contains(xmlStr, `<feed xmlns="http://www.w3.org/2005/Atom">`) {
		t.Errorf("missing Atom feed namespace")
	}
	if !strings.Contains(xmlStr, "<title>National Geographic (@natgeo) - INSTAGRAM</title>") {
		t.Errorf("title not rendered properly in XML: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<link href="http://localhost:9527/feeds/instagram/natgeo-feed.xml" rel="self" type="application/atom+xml"></link>`) {
		t.Errorf("self link not found in XML")
	}

	// Check entry contents
	if !strings.Contains(xmlStr, `<id>https://www.instagram.com/p/post-123/</id>`) {
		t.Errorf("entry ID not found")
	}
	// Verify thumbnail image tag is embedded into HTML content
	if !strings.Contains(xmlStr, `<img src="https://scontent.cdninstagram.com/v/sample.jpg"`) {
		t.Errorf("thumbnail img tag missing from content: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "A breathtaking view of the mountains.") {
		t.Errorf("caption text missing from entry content")
	}

	// Verify parse roundtrip
	parsed, err := ParseAtomFeed(xmlData)
	if err != nil {
		t.Fatalf("failed to parse generated Atom XML: %v", err)
	}

	if len(parsed.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed.Entries))
	}
	if parsed.Entries[0].ID != "https://www.instagram.com/p/post-123/" {
		t.Errorf("got entry ID %q, want https://www.instagram.com/p/post-123/", parsed.Entries[0].ID)
	}
}
