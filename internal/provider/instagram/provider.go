package instagram

import (
	"context"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/provider"
)

func init() {
	provider.Register(NewProvider())
}

// Provider implements provider.PlatformProvider for Instagram using a headless browser.
type Provider struct {
	browser *BrowserClient
}

// NewProvider creates a new Instagram Provider instance.
func NewProvider() *Provider {
	return &Provider{
		browser: NewBrowserClient(),
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

// FetchPosts fetches latest posts for an Instagram profile via Headless Chrome.
func (p *Provider) FetchPosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error) {
	return p.browser.FetchProfilePosts(ctx, profile, sinceID)
}

// Close releases browser resources.
func (p *Provider) Close() error {
	p.browser.Close()
	return nil
}
