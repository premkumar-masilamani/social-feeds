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
	if !strings.Contains(xmlStr, "<title>National Geographic (@natgeo)</title>") {
		t.Errorf("title not rendered properly in XML: %s", xmlStr)
	}
	if strings.Contains(xmlStr, "INSTAGRAM") {
		t.Errorf("feed title should not contain platform suffix INSTAGRAM: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<link href="http://localhost:9527/feeds/instagram/natgeo-feed.xml" rel="self" type="application/atom+xml"></link>`) {
		t.Errorf("self link not found in XML")
	}

	// Check entry contents
	if !strings.Contains(xmlStr, `<id>https://www.instagram.com/p/post-123/</id>`) {
		t.Errorf("entry ID not found")
	}
	// Verify thumbnail image tag is wrapped in an anchor link to make it clickable
	if !strings.Contains(xmlStr, `<a href="https://www.instagram.com/p/post-123/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/sample.jpg" alt="Photo"`) {
		t.Errorf("clickable thumbnail img tag missing from content: %s", xmlStr)
	}
	if strings.Contains(xmlStr, "View post on") {
		t.Errorf("did not expect separate 'View post on' text link when thumbnail is present: %s", xmlStr)
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

func TestCleanEntryTitleAndContent(t *testing.T) {
	fixedTime := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		post          model.Post
		expectedTitle string
		expectedDesc  string
		expectInHTML  string
		expectNotIn   string
	}{
		{
			post: model.Post{
				URL:          "https://www.instagram.com/reel/DcFuLeATEc9/",
				Caption:      "Video by Samyuktha Ranilakshmi on August 15, 2026. May be an image of child, dancing, smiling and text.",
				ThumbnailURL: "https://scontent.cdninstagram.com/v/sample.jpg",
				PublishedAt:  fixedTime,
				IsVideo:      true,
			},
			expectedTitle: "Video",
			expectedDesc:  "",
			expectInHTML:  `<a href="https://www.instagram.com/reel/DcFuLeATEc9/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/sample.jpg" alt="Video"`,
			expectNotIn:   "View post on",
		},
		{
			post: model.Post{
				URL:          "https://www.instagram.com/p/xyz/",
				Caption:      "Photo by Samyuktha Ranilakshmi on October 01, 2022.",
				ThumbnailURL: "https://scontent.cdninstagram.com/v/sample_photo.jpg",
				PublishedAt:  fixedTime,
				IsVideo:      false,
			},
			expectedTitle: "Photo",
			expectedDesc:  "",
			expectInHTML:  `<a href="https://www.instagram.com/p/xyz/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/sample_photo.jpg" alt="Photo"`,
			expectNotIn:   "View post on",
		},
		{
			post: model.Post{
				URL:          "https://www.instagram.com/p/nature123/",
				Caption:      "A day in the woods enjoying nature.",
				ThumbnailURL: "https://scontent.cdninstagram.com/v/nature.jpg",
				PublishedAt:  fixedTime,
				IsVideo:      false,
			},
			expectedTitle: "Photo",
			expectedDesc:  "A day in the woods enjoying nature.",
			expectInHTML:  `<a href="https://www.instagram.com/p/nature123/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/nature.jpg" alt="Photo"`,
			expectNotIn:   "View post on",
		},
		{
			post: model.Post{
				URL:         "https://www.instagram.com/reel/video123/",
				Caption:     "Surfing the biggest wave of the summer!",
				PublishedAt: fixedTime,
				IsVideo:     true,
			},
			expectedTitle: "Video",
			expectedDesc:  "Surfing the biggest wave of the summer!",
			expectInHTML:  "<p>Surfing the biggest wave of the summer!</p>",
			expectNotIn:   "May be an image of",
		},
		{
			post: model.Post{
				URL:         "https://www.instagram.com/p/empty123/",
				Caption:     "",
				PublishedAt: fixedTime,
				IsVideo:     false,
			},
			expectedTitle: "Photo",
			expectedDesc:  "",
			expectInHTML:  "View post on instagram",
			expectNotIn:   "<p></p>",
		},
		{
			post: model.Post{
				URL:          "https://www.instagram.com/reel/DZhqagnOnFNGYq7YKn414FStUGtD2pnz8obJmg0/",
				Caption:      "Video by Sam on June 13, 2026.",
				ThumbnailURL: "https://scontent.cdninstagram.com/v/collab.jpg",
				PublishedAt:  fixedTime,
				IsVideo:      true,
				IsCollab:     true,
			},
			expectedTitle: "Collab Video",
			expectedDesc:  "",
			expectInHTML:  `<a href="https://www.instagram.com/reel/DZhqagnOnFNGYq7YKn414FStUGtD2pnz8obJmg0/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/collab.jpg" alt="Collab Video"`,
			expectNotIn:   "View post on",
		},
		{
			post: model.Post{
				URL:          "https://www.instagram.com/reel/DUTGRLQD7xVEWi8L3Fjwe1deYx7P1lQoM2NV5c0/",
				Caption:      "",
				ThumbnailURL: "https://scontent.cdninstagram.com/v/collab2.jpg",
				PublishedAt:  fixedTime,
				IsVideo:      true,
			},
			expectedTitle: "Collab Video",
			expectedDesc:  "",
			expectInHTML:  `<a href="https://www.instagram.com/reel/DUTGRLQD7xVEWi8L3Fjwe1deYx7P1lQoM2NV5c0/" target="_blank" rel="noopener noreferrer"><img src="https://scontent.cdninstagram.com/v/collab2.jpg" alt="Collab Video"`,
			expectNotIn:   "View post on",
		},
	}

	profile := &model.Profile{Platform: "instagram", Handle: "myu_stories"}

	for _, tt := range tests {
		title := getEntryTitle(tt.post)
		if title != tt.expectedTitle {
			t.Errorf("post %s: got title %q, want %q", tt.post.URL, title, tt.expectedTitle)
		}

		desc := extractRealDescription(tt.post.Caption)
		if desc != tt.expectedDesc {
			t.Errorf("post %s: got desc %q, want %q", tt.post.URL, desc, tt.expectedDesc)
		}

		htmlContent := buildContentHTML(profile, tt.post, desc)
		if !strings.Contains(htmlContent, tt.expectInHTML) {
			t.Errorf("post %s: expected HTML to contain %q, got: %s", tt.post.URL, tt.expectInHTML, htmlContent)
		}
		if tt.expectNotIn != "" && strings.Contains(htmlContent, tt.expectNotIn) {
			t.Errorf("post %s: did not expect HTML to contain %q, got: %s", tt.post.URL, tt.expectNotIn, htmlContent)
		}
	}
}

func TestFeedTitleWithoutPlatform(t *testing.T) {
	// Profile without FullName
	p1 := &model.Profile{
		Platform: "instagram",
		Handle:   "myu_stories",
		URL:      "https://www.instagram.com/myu_stories/",
	}
	xml1, err := GenerateAtomXML(p1, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(xml1), "<title>@myu_stories</title>") {
		t.Errorf("expected <title>@myu_stories</title>, got %s", string(xml1))
	}
	if strings.Contains(string(xml1), "INSTAGRAM") {
		t.Errorf("should not contain INSTAGRAM: %s", string(xml1))
	}

	// Profile with FullName identical to Handle
	p2 := &model.Profile{
		Platform: "instagram",
		Handle:   "nasa",
		FullName: "nasa",
		URL:      "https://www.instagram.com/nasa/",
	}
	xml2, err := GenerateAtomXML(p2, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(xml2), "<title>@nasa</title>") {
		t.Errorf("expected <title>@nasa</title>, got %s", string(xml2))
	}

	// Profile with FullName distinct from Handle
	p3 := &model.Profile{
		Platform: "instagram",
		Handle:   "natgeo",
		FullName: "National Geographic",
		URL:      "https://www.instagram.com/natgeo/",
	}
	xml3, err := GenerateAtomXML(p3, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(xml3), "<title>National Geographic (@natgeo)</title>") {
		t.Errorf("expected <title>National Geographic (@natgeo)</title>, got %s", string(xml3))
	}
	if strings.Contains(string(xml3), "INSTAGRAM") {
		t.Errorf("should not contain INSTAGRAM: %s", string(xml3))
	}
}
