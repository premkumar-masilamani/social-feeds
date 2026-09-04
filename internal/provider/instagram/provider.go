package instagram

import (
	"context"
	"fmt"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/provider"
)

func init() {
	provider.Register(NewProvider())
}

// Provider implements provider.PlatformProvider for Instagram.
type Provider struct {
	client *Client
}

// NewProvider creates a new Instagram Provider instance.
func NewProvider(opts ...ClientOption) *Provider {
	return &Provider{
		client: NewClient(opts...),
	}
}

// Name returns the platform identifier.
func (p *Provider) Name() string {
	return "instagram"
}

// SourceFile returns the expected input file name.
func (p *Provider) SourceFile() string {
	return "instagram.txt"
}

// ParseTarget parses a line from instagram.txt.
func (p *Provider) ParseTarget(line string) (*model.Profile, error) {
	return ParseTarget(line)
}

// FetchPosts fetches latest posts for an Instagram profile.
func (p *Provider) FetchPosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error) {
	// Respect polite wait between profile requests
	p.client.PoliteWait(ctx)

	resp, err := p.client.FetchProfileData(ctx, profile.Handle)
	if err != nil {
		return nil, err
	}

	user := resp.Data.User
	if profile.FullName == "" {
		profile.FullName = user.FullName
	}
	if profile.Bio == "" {
		profile.Bio = user.Biography
	}

	edges := user.EdgeOwnerToTimelineMedia.Edges
	posts := make([]model.Post, 0, len(edges))

	for _, edge := range edges {
		node := edge.Node
		postID := node.Shortcode
		if postID == "" {
			postID = node.ID
		}

		// Check if we reached the most recent known post
		if sinceID != "" && postID == sinceID {
			break
		}

		caption := ""
		if len(node.EdgeMediaToCaption.Edges) > 0 {
			caption = node.EdgeMediaToCaption.Edges[0].Node.Text
		}

		postURL := fmt.Sprintf("https://www.instagram.com/p/%s/", node.Shortcode)
		pubTime := time.Unix(node.TakenAtTimestamp, 0)
		if node.TakenAtTimestamp == 0 {
			pubTime = time.Now()
		}

		posts = append(posts, model.Post{
			ID:           postID,
			URL:          postURL,
			Caption:      caption,
			ThumbnailURL: node.DisplayURL,
			PublishedAt:  pubTime,
			Author:       user.Username,
			IsVideo:      node.IsVideo,
		})
	}

	return posts, nil
}
