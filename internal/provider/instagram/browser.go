package instagram

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

var (
	dateRegex      = regexp.MustCompile(`(?i)(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),\s+(\d{4})`)
	shortcodeRegex = regexp.MustCompile(`/(?:p|reel)/([A-Za-z0-9_-]+)`)
)

// RawDOMPost represents raw post data extracted from the browser DOM.
type RawDOMPost struct {
	HRef    string `json:"href"`
	ImgURL  string `json:"img_url"`
	Caption string `json:"caption"`
}

// BrowserClient handles automated headless scraping of Instagram.
type BrowserClient struct {
	userDataDir   string
	headless      bool
	username      string
	password      string
	mu            sync.Mutex
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	isLoggedIn    bool
}

// NewBrowserClient initializes a headless browser client.
func NewBrowserClient() *BrowserClient {
	username := strings.TrimSpace(os.Getenv("INSTAGRAM_USERNAME"))
	password := strings.TrimSpace(os.Getenv("INSTAGRAM_PASSWORD"))

	userDataDir := os.Getenv("BROWSER_DATA_DIR")
	if userDataDir == "" {
		userDataDir = filepath.Join(".", ".browser_profile")
	}

	return &BrowserClient{
		userDataDir: userDataDir,
		headless:    true,
		username:    username,
		password:    password,
	}
}

func (b *BrowserClient) getCredentials() (string, string) {
	u := b.username
	if u == "" {
		u = strings.TrimSpace(os.Getenv("INSTAGRAM_USERNAME"))
	}
	p := b.password
	if p == "" {
		p = strings.TrimSpace(os.Getenv("INSTAGRAM_PASSWORD"))
	}
	return u, p
}

func (b *BrowserClient) ensureBrowser(parentCtx context.Context) (context.Context, error) {
	if b.browserCtx != nil && b.browserCtx.Err() == nil {
		return b.browserCtx, nil
	}

	if err := os.MkdirAll(b.userDataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating browser user data dir: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(b.userDataDir),
		chromedp.Flag("headless", b.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"),
	)

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(parentCtx, opts...)
	b.browserCtx, b.browserCancel = chromedp.NewContext(b.allocCtx, chromedp.WithLogf(log.Printf))

	return b.browserCtx, nil
}

// Close gracefully releases headless browser resources.
func (b *BrowserClient) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.browserCancel != nil {
		b.browserCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
	b.browserCtx = nil
	b.allocCtx = nil
	b.isLoggedIn = false
}

// Login performs an automated login to Instagram if credentials are provided and session is unauthenticated.
func (b *BrowserClient) Login(ctx context.Context) error {
	username, password := b.getCredentials()
	if username == "" || password == "" {
		log.Println("[Browser] Warning: INSTAGRAM_USERNAME or INSTAGRAM_PASSWORD not configured in .env; proceeding in guest mode")
		return nil
	}

	browserCtx, err := b.ensureBrowser(ctx)
	if err != nil {
		return err
	}

	var currentURL, pageTitle string
	log.Println("[Browser] Checking Instagram authentication status...")

	err = chromedp.Run(browserCtx,
		chromedp.Navigate("https://www.instagram.com/"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
		chromedp.Title(&pageTitle),
	)
	if err != nil {
		return fmt.Errorf("loading homepage: %w", err)
	}

	// Check if already logged in
	if !strings.Contains(currentURL, "/accounts/login") && !strings.Contains(currentURL, "/accounts/emailsignup") {
		log.Println("[Browser] Active logged-in session detected in browser profile.")
		b.isLoggedIn = true
		return nil
	}

	log.Printf("[Browser] Logging into Instagram as @%s...", username)
	loginAction := chromedp.Tasks{
		chromedp.Navigate("https://www.instagram.com/accounts/login/"),
		chromedp.Sleep(2 * time.Second),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="username"]`, username, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(6 * time.Second),
		chromedp.Location(&currentURL),
	}

	if err := chromedp.Run(browserCtx, loginAction); err != nil {
		return fmt.Errorf("login submission failed: %w", err)
	}

	log.Printf("[Browser] Post-login URL: %s", currentURL)
	if strings.Contains(currentURL, "/accounts/login") {
		return fmt.Errorf("Instagram login failed; check INSTAGRAM_USERNAME and INSTAGRAM_PASSWORD in .env")
	}

	b.isLoggedIn = true
	log.Println("[Browser] Logged in successfully.")
	return nil
}

// FetchProfilePosts navigates to the public Instagram profile and extracts posts from the DOM.
func (b *BrowserClient) FetchProfilePosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	browserCtx, err := b.ensureBrowser(ctx)
	if err != nil {
		return nil, err
	}

	// Attempt login if not already done
	if !b.isLoggedIn {
		if err := b.Login(browserCtx); err != nil {
			log.Printf("[Browser] Login attempt warning: %v", err)
		}
	}

	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", profile.Handle)
	log.Printf("[Browser] Navigating to %s ...", profileURL)

	var pageTitle, pageHTML string
	var rawItems []RawDOMPost

	extractScript := `
		(() => {
			const items = [];
			const seen = new Set();
			const anchors = document.querySelectorAll('a[href*="/p/"], a[href*="/reel/"]');
			for (const a of anchors) {
				const href = a.getAttribute('href');
				if (!href || seen.has(href)) continue;
				seen.add(href);
				const img = a.querySelector('img');
				items.push({
					href: href,
					img_url: img ? (img.src || img.getAttribute('src') || '') : '',
					caption: img ? (img.alt || img.getAttribute('alt') || '') : ''
				});
			}
			return items;
		})()
	`

	navigateTasks := chromedp.Tasks{
		chromedp.Navigate(profileURL),
		chromedp.Sleep(4 * time.Second),
		chromedp.Title(&pageTitle),
		chromedp.Evaluate(extractScript, &rawItems),
		chromedp.OuterHTML("html", &pageHTML),
	}

	if err := chromedp.Run(browserCtx, navigateTasks); err != nil {
		return nil, fmt.Errorf("fetching profile %s via browser: %w", profile.Handle, err)
	}

	if strings.Contains(strings.ToLower(pageTitle), "page not found") {
		return nil, fmt.Errorf("profile @%s not found on Instagram", profile.Handle)
	}

	// Update profile metadata if extracted
	if profile.FullName == "" && strings.Contains(pageTitle, "(") {
		parts := strings.Split(pageTitle, "(")
		if len(parts) > 0 {
			profile.FullName = strings.TrimSpace(parts[0])
		}
	}

	posts := make([]model.Post, 0, len(rawItems))

	for _, item := range rawItems {
		shortcode := ""
		matches := shortcodeRegex.FindStringSubmatch(item.HRef)
		if len(matches) > 1 {
			shortcode = matches[1]
		}
		if shortcode == "" {
			continue
		}

		if sinceID != "" && shortcode == sinceID {
			break
		}

		postURL := fmt.Sprintf("https://www.instagram.com/p/%s/", shortcode)
		if strings.Contains(item.HRef, "/reel/") {
			postURL = fmt.Sprintf("https://www.instagram.com/reel/%s/", shortcode)
		}

		pubTime := parseCaptionDate(item.Caption)
		if pubTime.IsZero() {
			pubTime = time.Now()
		}

		posts = append(posts, model.Post{
			ID:           shortcode,
			URL:          postURL,
			Caption:      item.Caption,
			ThumbnailURL: item.ImgURL,
			PublishedAt:  pubTime,
			Author:       profile.Handle,
			IsVideo:      strings.Contains(item.HRef, "/reel/"),
		})
	}

	log.Printf("[Browser] Extracted %d posts for @%s", len(posts), profile.Handle)
	return posts, nil
}

func parseCaptionDate(caption string) time.Time {
	m := dateRegex.FindStringSubmatch(caption)
	if len(m) == 4 {
		monthStr := m[1]
		dayStr := m[2]
		yearStr := m[3]
		dateStr := fmt.Sprintf("%s %s, %s", monthStr, dayStr, yearStr)
		if t, err := time.Parse("January 2, 2006", dateStr); err == nil {
			return t
		}
	}
	return time.Time{}
}
