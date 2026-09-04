package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	webProfileInfoURL = "https://www.instagram.com/api/v1/users/web_profile_info/?username=%s"
	igAppID           = "936619743392459"
	defaultUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
)

// HTTPClient interface for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client manages polite HTTP scraping against Instagram endpoints.
type Client struct {
	httpClient HTTPClient
	sessionID  string
	userAgent  string
	minDelay   time.Duration
	maxDelay   time.Duration
}

// ClientOption allows configuring client behaviors.
type ClientOption func(*Client)

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(c HTTPClient) ClientOption {
	return func(client *Client) {
		client.httpClient = c
	}
}

// WithDelays configures polite request delays between requests.
func WithDelays(min, max time.Duration) ClientOption {
	return func(client *Client) {
		client.minDelay = min
		client.maxDelay = max
	}
}

// NewClient creates a new Instagram HTTP client.
func NewClient(opts ...ClientOption) *Client {
	sessionID := strings.TrimSpace(os.Getenv("INSTAGRAM_SESSION_ID"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(os.Getenv("INSTAGRAM_COOKIE"))
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		sessionID: sessionID,
		userAgent: defaultUserAgent,
		minDelay:  3 * time.Second,
		maxDelay:  6 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// PoliteWait pauses execution with randomized jitter to respect rate limits.
func (c *Client) PoliteWait(ctx context.Context) {
	if c.maxDelay <= 0 {
		return
	}
	diff := c.maxDelay - c.minDelay
	var sleepDuration time.Duration
	if diff > 0 {
		sleepDuration = c.minDelay + time.Duration(rand.Int63n(int64(diff)))
	} else {
		sleepDuration = c.minDelay
	}

	select {
	case <-time.After(sleepDuration):
	case <-ctx.Done():
	}
}

// WebProfileResponse represents Instagram's web_profile_info response structure.
type WebProfileResponse struct {
	Data struct {
		User struct {
			ID                       string `json:"id"`
			Username                 string `json:"username"`
			FullName                 string `json:"full_name"`
			Biography                string `json:"biography"`
			ProfilePicURLHD          string `json:"profile_pic_url_hd"`
			IsPrivate                bool   `json:"is_private"`
			EdgeOwnerToTimelineMedia struct {
				Count int `json:"count"`
				Edges []struct {
					Node struct {
						ID                 string `json:"id"`
						Shortcode          string `json:"shortcode"`
						DisplayURL         string `json:"display_url"`
						IsVideo            bool   `json:"is_video"`
						TakenAtTimestamp   int64  `json:"taken_at_timestamp"`
						EdgeMediaToCaption struct {
							Edges []struct {
								Node struct {
									Text string `json:"text"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"edge_media_to_caption"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"edge_owner_to_timeline_media"`
		} `json:"user"`
	} `json:"data"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// FetchProfileData performs an HTTP request to fetch Instagram user metadata and timeline media.
func (c *Client) FetchProfileData(ctx context.Context, username string) (*WebProfileResponse, error) {
	endpoint := fmt.Sprintf(webProfileInfoURL, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-IG-App-ID", igAppID)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Referer", fmt.Sprintf("https://www.instagram.com/%s/", username))

	if c.sessionID != "" {
		if strings.Contains(c.sessionID, "=") {
			req.Header.Set("Cookie", c.sessionID)
		} else {
			req.Header.Set("Cookie", fmt.Sprintf("sessionid=%s", c.sessionID))
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request for @%s: %w", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (HTTP 429) by Instagram when fetching @%s; try increasing poll frequency or setting INSTAGRAM_SESSION_ID", username)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("access denied (HTTP %d) by Instagram for @%s (login wall); set INSTAGRAM_SESSION_ID env var to authenticate", resp.StatusCode, username)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected HTTP %d from Instagram for @%s: %s", resp.StatusCode, username, string(body))
	}

	var data WebProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding JSON for @%s: %w", username, err)
	}

	if data.Data.User.Username == "" {
		return nil, fmt.Errorf("user @%s not found or empty profile returned", username)
	}

	if data.Data.User.IsPrivate {
		return nil, fmt.Errorf("account @%s is private", username)
	}

	return &data, nil
}
