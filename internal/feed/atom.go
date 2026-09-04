package feed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

// AtomFeed represents the root XML structure of an Atom 1.0 feed.
type AtomFeed struct {
	XMLName  xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Links    []AtomLink  `xml:"link"`
	Author   *AtomAuthor `xml:"author,omitempty"`
	Entries  []AtomEntry `xml:"entry"`
}

// AtomLink represents an Atom link tag.
type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

// AtomAuthor represents the feed/entry author.
type AtomAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri,omitempty"`
}

// AtomContent holds the HTML content element.
type AtomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",cdata"`
}

// AtomEntry represents an individual item in the Atom feed.
type AtomEntry struct {
	Title     string       `xml:"title"`
	Link      AtomLink     `xml:"link"`
	ID        string       `xml:"id"`
	Published string       `xml:"published"`
	Updated   string       `xml:"updated"`
	Author    *AtomAuthor  `xml:"author,omitempty"`
	Summary   string       `xml:"summary,omitempty"`
	Content   *AtomContent `xml:"content,omitempty"`
}

// GenerateAtomXML generates an Atom 1.0 XML document bytes for a profile and slice of posts.
func GenerateAtomXML(profile *model.Profile, posts []model.Post, feedSelfURL string) ([]byte, error) {
	updatedTime := time.Now().UTC()
	if len(posts) > 0 && !posts[0].PublishedAt.IsZero() {
		updatedTime = posts[0].PublishedAt.UTC()
	}

	title := fmt.Sprintf("%s (@%s)", profile.Handle, profile.Handle)
	if profile.FullName != "" {
		title = fmt.Sprintf("%s (@%s) - %s", profile.FullName, profile.Handle, strings.ToUpper(profile.Platform))
	} else {
		title = fmt.Sprintf("@%s - %s Feed", profile.Handle, strings.ToUpper(profile.Platform))
	}

	atomFeed := AtomFeed{
		Title:    title,
		Subtitle: profile.Bio,
		ID:       profile.URL,
		Updated:  updatedTime.Format(time.RFC3339),
		Links: []AtomLink{
			{Href: profile.URL, Rel: "alternate", Type: "text/html"},
		},
		Author: &AtomAuthor{
			Name: profile.Handle,
			URI:  profile.URL,
		},
		Entries: make([]AtomEntry, 0, len(posts)),
	}

	if feedSelfURL != "" {
		atomFeed.Links = append(atomFeed.Links, AtomLink{
			Href: feedSelfURL,
			Rel:  "self",
			Type: "application/atom+xml",
		})
	}

	for _, post := range posts {
		entry := buildAtomEntry(profile, post)
		atomFeed.Entries = append(atomFeed.Entries, entry)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	if err := encoder.Encode(atomFeed); err != nil {
		return nil, fmt.Errorf("encoding atom feed: %w", err)
	}
	buf.WriteString("\n")

	return buf.Bytes(), nil
}

var (
	metaAltRegex = regexp.MustCompile(`(?i)^(Photo|Video|Reel)\s+by\s+(.*?)\s+on\s+([A-Za-z]+\s+\d{1,2},\s+\d{4})`)
	mayBeRegex   = regexp.MustCompile(`(?i)\.?\s*May be an?\s+(?:image|illustration|audio)\s+of\s*([^.]*)\.?`)
)

func buildAtomEntry(profile *model.Profile, post model.Post) AtomEntry {
	pubTime := post.PublishedAt.UTC()
	if pubTime.IsZero() {
		pubTime = time.Now().UTC()
	}
	timeStr := pubTime.Format(time.RFC3339)

	entryTitle := getEntryTitle(post)
	description := extractRealDescription(post.Caption)
	contentHTML := buildContentHTML(profile, post, description)

	return AtomEntry{
		Title:     entryTitle,
		Link:      AtomLink{Href: post.URL, Rel: "alternate", Type: "text/html"},
		ID:        post.URL,
		Published: timeStr,
		Updated:   timeStr,
		Author: &AtomAuthor{
			Name: post.Author,
			URI:  profile.URL,
		},
		Summary: description,
		Content: &AtomContent{
			Type:  "html",
			Value: contentHTML,
		},
	}
}

// getEntryTitle returns simply "Video" or "Photo".
func getEntryTitle(post model.Post) string {
	if post.IsVideo || strings.Contains(strings.ToLower(post.URL), "/reel/") || strings.HasPrefix(strings.ToLower(post.Caption), "video") || strings.HasPrefix(strings.ToLower(post.Caption), "reel") {
		return "Video"
	}
	return "Photo"
}

// extractRealDescription extracts genuine user captions, discarding Meta's computer vision alt text.
func extractRealDescription(caption string) string {
	if caption == "" {
		return ""
	}
	// If it matches Meta's accessibility alt text (e.g. "Photo by ... on ... May be an image of ...")
	if metaAltRegex.MatchString(caption) {
		return ""
	}
	// Strip any standalone "May be an image of..." boilerplate if present
	cleaned := mayBeRegex.ReplaceAllString(caption, "")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

// buildContentHTML formats post body into rich HTML with a clickable thumbnail and real descriptions if present.
func buildContentHTML(profile *model.Profile, post model.Post, description string) string {
	var contentHTML strings.Builder
	if post.ThumbnailURL != "" {
		altText := getEntryTitle(post)
		contentHTML.WriteString(fmt.Sprintf(
			`<p><a href="%s" target="_blank" rel="noopener noreferrer"><img src="%s" alt="%s" style="max-width: 100%%; border-radius: 8px;" /></a></p>`,
			html.EscapeString(post.URL),
			html.EscapeString(post.ThumbnailURL),
			html.EscapeString(altText),
		))
	} else if post.URL != "" {
		contentHTML.WriteString(fmt.Sprintf(
			`<p><a href="%s" target="_blank" rel="noopener noreferrer">View post on %s</a></p>`,
			html.EscapeString(post.URL),
			html.EscapeString(profile.Platform),
		))
	}
	if description != "" {
		escaped := html.EscapeString(description)
		paragraphs := strings.Split(escaped, "\n\n")
		for _, p := range paragraphs {
			lineBreaks := strings.ReplaceAll(p, "\n", "<br />")
			contentHTML.WriteString(fmt.Sprintf("<p>%s</p>", lineBreaks))
		}
	}
	return contentHTML.String()
}

// ParseAtomFeed parses raw Atom XML bytes into an AtomFeed struct.
func ParseAtomFeed(data []byte) (*AtomFeed, error) {
	var f AtomFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing atom feed XML: %w", err)
	}
	return &f, nil
}
