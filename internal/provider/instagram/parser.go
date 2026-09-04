package instagram

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

var (
	// Instagram usernames contain letters, numbers, periods, and underscores, up to 30 chars.
	validUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._]{1,30}$`)
)

// ParseTarget parses a line from the handle file into a normalized Profile.
// Only pure handle names are supported (e.g. "username" or "@username").
// Full URLs are rejected with an informative error instructing the user to supply only the handle name.
func ParseTarget(line string) (*model.Profile, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil, errors.New("empty or comment line")
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.Contains(trimmed, "/") || strings.Contains(trimmed, ":") {
		return nil, fmt.Errorf("invalid handle %q: only handle names (e.g. 'username' or '@username') are supported; please extract the handle name from the URL", trimmed)
	}

	username := strings.TrimPrefix(trimmed, "@")
	username = strings.TrimSpace(username)

	if !validUsernameRegex.MatchString(username) {
		return nil, fmt.Errorf("invalid Instagram handle %q: must contain 1-30 letters, numbers, periods, or underscores", username)
	}

	// Skip reserved keywords
	switch strings.ToLower(username) {
	case "explore", "reels", "stories", "direct", "accounts", "p", "tv":
		return nil, fmt.Errorf("reserved keyword %q is not a valid handle", username)
	}

	return &model.Profile{
		Platform: "instagram",
		Handle:   username,
		URL:      fmt.Sprintf("https://www.instagram.com/%s/", username),
	}, nil
}
