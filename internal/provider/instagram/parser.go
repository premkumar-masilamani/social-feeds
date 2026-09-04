package instagram

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

var (
	// Instagram usernames contain letters, numbers, periods, and underscores, up to 30 chars.
	validUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._]{1,30}$`)
)

// ParseTarget parses a line from instagram.txt into a normalized Profile.
// Supported formats:
// - https://www.instagram.com/natgeo/
// - http://instagram.com/natgeo
// - @natgeo
// - natgeo
func ParseTarget(line string) (*model.Profile, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil, errors.New("empty or comment line")
	}

	var username string

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", trimmed, err)
		}
		path := strings.Trim(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			return nil, fmt.Errorf("could not extract username from URL %q", trimmed)
		}
		// Skip reserved keywords
		switch strings.ToLower(parts[0]) {
		case "explore", "reels", "stories", "direct", "accounts", "p", "tv":
			return nil, fmt.Errorf("URL %q is not a profile URL", trimmed)
		default:
			username = parts[0]
		}
	} else {
		username = strings.TrimPrefix(trimmed, "@")
		username = strings.Trim(username, "/")
	}

	username = strings.TrimSpace(username)
	if !validUsernameRegex.MatchString(username) {
		return nil, fmt.Errorf("invalid Instagram username %q", username)
	}

	return &model.Profile{
		Platform: "instagram",
		Handle:   username,
		URL:      fmt.Sprintf("https://www.instagram.com/%s/", username),
	}, nil
}
