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

// GetPlatformSourceFiles returns all existing source files (public, .local, .private, or subdirectories)
// for the given platform in handlesDir.
func GetPlatformSourceFiles(handlesDir, platformName string) []string {
	candidates := []string{
		filepath.Join(handlesDir, platformName+".txt"),
		filepath.Join(handlesDir, platformName+".local.txt"),
		filepath.Join(handlesDir, platformName+".private.txt"),
		filepath.Join(handlesDir, "private", platformName+".txt"),
		filepath.Join(handlesDir, "local", platformName+".txt"),
	}

	var found []string
	seen := make(map[string]bool)
	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			if !seen[path] {
				seen[path] = true
				found = append(found, path)
			}
		}
	}

	// Fallback to project root if handlesDir is different and no files found
	if len(found) == 0 && handlesDir != "." {
		rootFallback := filepath.Join(".", platformName+".txt")
		if fi, err := os.Stat(rootFallback); err == nil && !fi.IsDir() {
			found = append(found, rootFallback)
		}
	}

	return found
}

// DiscoverActiveProviders scans handlesDir and returns all registered providers
// that have at least one valid source file (e.g. handles/instagram.txt, handles/instagram.local.txt).
func DiscoverActiveProviders(handlesDir string) []PlatformProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var active []PlatformProvider
	for _, p := range providers {
		files := GetPlatformSourceFiles(handlesDir, p.Name())
		if len(files) > 0 {
			active = append(active, p)
		}
	}
	return active
}
