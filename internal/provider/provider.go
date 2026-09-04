package provider

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

// PlatformProvider defines the interface for fetching posts from social media platforms.
type PlatformProvider interface {
	// Name returns the identifier of the platform (e.g., "instagram", "facebook", "x").
	Name() string
	// SourceFile returns the input filename (e.g., "instagram.txt").
	SourceFile() string
	// ParseTarget converts a single line from the source file into a normalized Profile.
	ParseTarget(line string) (*model.Profile, error)
	// FetchPosts retrieves posts for the given profile since sinceID.
	FetchPosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error)
}

var (
	registryMu sync.RWMutex
	providers  = make(map[string]PlatformProvider)
)

// Register registers a platform provider.
func Register(p PlatformProvider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	providers[p.Name()] = p
}

// GetProvider retrieves a provider by platform name.
func GetProvider(name string) (PlatformProvider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// AllProviders returns a slice of all registered providers.
func AllProviders() []PlatformProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	list := make([]PlatformProvider, 0, len(providers))
	for _, p := range providers {
		list = append(list, p)
	}
	return list
}

// DiscoverActiveProviders scans baseDir and returns all registered providers
// whose corresponding source file (e.g., instagram.txt) exists on disk.
func DiscoverActiveProviders(baseDir string) []PlatformProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var active []PlatformProvider
	for _, p := range providers {
		sourcePath := filepath.Join(baseDir, p.SourceFile())
		if fi, err := os.Stat(sourcePath); err == nil && !fi.IsDir() {
			active = append(active, p)
		}
	}
	return active
}
