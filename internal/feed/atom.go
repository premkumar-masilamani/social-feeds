package feed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
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

func buildAtomEntry(profile *model.Profile, post model.Post) AtomEntry {
	pubTime := post.PublishedAt.UTC()
	if pubTime.IsZero() {
		pubTime = time.Now().UTC()
	}
	timeStr := pubTime.Format(time.RFC3339)

	// Create concise title snippet from caption or fallback to date
	entryTitle := post.Caption
	if entryTitle == "" {
		entryTitle = fmt.Sprintf("Post on %s", pubTime.Format("Jan 02, 2006"))
	} else {
		entryTitle = strings.ReplaceAll(entryTitle, "\n", " ")
		runes := []rune(entryTitle)
		if len(runes) > 90 {
			entryTitle = string(runes[:87]) + "..."
		}
	}

	// Build rich HTML content embedding the remote CDN image
	var contentHTML strings.Builder
	if post.ThumbnailURL != "" {
		contentHTML.WriteString(fmt.Sprintf(
			`<p><img src="%s" alt="Thumbnail" style="max-width: 100%%; border-radius: 8px;" /></p>`,
			html.EscapeString(post.ThumbnailURL),
		))
	}
	if post.Caption != "" {
		escapedCaption := html.EscapeString(post.Caption)
		paragraphs := strings.Split(escapedCaption, "\n\n")
		for _, p := range paragraphs {
			lineBreaks := strings.ReplaceAll(p, "\n", "<br />")
			contentHTML.WriteString(fmt.Sprintf("<p>%s</p>", lineBreaks))
		}
	}
	contentHTML.WriteString(fmt.Sprintf(
		`<p><a href="%s" target="_blank" rel="noopener noreferrer">View post on %s</a></p>`,
		html.EscapeString(post.URL),
		strings.Title(profile.Platform),
	))

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
		Content: &AtomContent{
			Type:  "html",
			Value: contentHTML.String(),
		},
	}
}

// ParseAtomFeed parses raw Atom XML bytes into an AtomFeed struct.
func ParseAtomFeed(data []byte) (*AtomFeed, error) {
	var f AtomFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing atom feed XML: %w", err)
	}
	return &f, nil
}
