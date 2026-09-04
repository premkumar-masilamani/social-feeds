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
	shortcodeRegex = regexp.MustCompile(`/(?:p|reels?)/([A-Za-z0-9_-]+)`)
)

// extractShortcode parses the post/reel identifier from an Instagram URL or href without modifying the raw identifier.
func extractShortcode(href string) string {
	matches := shortcodeRegex.FindStringSubmatch(href)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// RawDOMPost represents raw post data extracted from the browser DOM.
type RawDOMPost struct {
	HRef    string `json:"href"`
	ImgURL  string `json:"img_url"`
	Caption string `json:"caption"`
}

// PageLoadStatus holds the outcome of checking the profile DOM during loading.
type PageLoadStatus struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	PostCount  int    `json:"post_count"`
	IsLogin    bool   `json:"is_login"`
	IsNotFound bool   `json:"is_not_found"`
	IsPrivate  bool   `json:"is_private"`
	IsNoPosts  bool   `json:"is_no_posts"`
}

// BrowserClient handles automated headless scraping of Instagram.
type BrowserClient struct {
	userDataDir   string
	headless      bool
	mu            sync.Mutex
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
}

// NewBrowserClient initializes a headless browser client using the local persistent profile.
func NewBrowserClient() *BrowserClient {
	userDataDir := os.Getenv("BROWSER_DATA_DIR")
	if userDataDir == "" {
		userDataDir = filepath.Join(".", ".browser_profile")
	}

	return &BrowserClient{
		userDataDir: userDataDir,
		headless:    true,
	}
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
		chromedp.Flag("window-size", "1280,900"),
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
}

// FetchProfilePosts navigates to the public Instagram profile and extracts posts from the DOM.
func (b *BrowserClient) FetchProfilePosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	browserCtx, err := b.ensureBrowser(ctx)
	if err != nil {
		return nil, err
	}

	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", profile.Handle)
	log.Printf("[Browser] Navigating to %s ...", profileURL)

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

	pageCheckScript := `
		(() => {
			const href = window.location.href || '';
			const title = document.title || '';
			const body = document.body;
			const bodyText = body ? (body.innerText || '') : '';
			const lowerHref = href.toLowerCase();
			const lowerTitle = title.toLowerCase();
			const lowerBody = bodyText.toLowerCase();

			const isLogin = lowerHref.includes('/accounts/login') ||
			                lowerHref.includes('is_from_rle') ||
			                lowerBody.includes('log in to instagram') ||
			                lowerTitle.startsWith('login • instagram');

			const isNotFound = lowerTitle.includes('page not found') ||
			                   lowerBody.includes("sorry, this page isn't available") ||
			                   lowerBody.includes('link you followed may be broken');

			const isPrivate = lowerBody.includes('this account is private') ||
			                  lowerBody.includes('this profile is private');

			const isNoPosts = lowerBody.includes('no posts yet') ||
			                  lowerBody.includes('no photos or videos yet');

			const anchors = document.querySelectorAll('a[href*="/p/"], a[href*="/reel/"]');

			return {
				url: href,
				title: title,
				post_count: anchors.length,
				is_login: isLogin,
				is_not_found: isNotFound,
				is_private: isPrivate,
				is_no_posts: isNoPosts
			};
		})()
	`

	if err := chromedp.Run(browserCtx, chromedp.Navigate(profileURL)); err != nil {
		return nil, fmt.Errorf("navigating to profile %s via browser: %w", profile.Handle, err)
	}

	// Wait dynamically up to 15 seconds for profile content to load
	waitCtx, waitCancel := context.WithTimeout(browserCtx, 15*time.Second)
	defer waitCancel()

	var loadStatus PageLoadStatus
	for {
		select {
		case <-waitCtx.Done():
			goto InitialWaitDone
		default:
		}

		time.Sleep(500 * time.Millisecond)

		var cur PageLoadStatus
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(pageCheckScript, &cur)); err != nil {
			continue
		}
		loadStatus = cur

		// Immediate error conditions
		if loadStatus.IsNotFound {
			return nil, fmt.Errorf("profile @%s not found on Instagram", profile.Handle)
		}
		if loadStatus.IsPrivate {
			return nil, fmt.Errorf("profile @%s is private; only public profiles can be syndicated", profile.Handle)
		}
		if loadStatus.IsLogin {
			return nil, fmt.Errorf("Instagram redirected to login wall / rate limit for @%s (url: %s); preserving existing feed", profile.Handle, loadStatus.URL)
		}

		// Success conditions
		if loadStatus.PostCount > 0 {
			// Stabilize for 1s to let images and child DOM nodes finish settling
			time.Sleep(1 * time.Second)
			break
		}
		if loadStatus.IsNoPosts {
			log.Printf("[Browser] Profile @%s explicitly has no posts", profile.Handle)
			break
		}
	}

InitialWaitDone:

	// If no posts were found and not confirmed empty, treat as load timeout
	if loadStatus.PostCount == 0 && !loadStatus.IsNoPosts {
		if loadStatus.IsLogin {
			return nil, fmt.Errorf("Instagram redirected to login wall / rate limit for @%s (url: %s); preserving existing feed", profile.Handle, loadStatus.URL)
		}
		return nil, fmt.Errorf("timeout waiting for page content to load for @%s (0 posts detected); preserving existing feed", profile.Handle)
	}

	// Update profile metadata if extracted
	if profile.FullName == "" && strings.Contains(loadStatus.Title, "(") {
		parts := strings.Split(loadStatus.Title, "(")
		if len(parts) > 0 {
			profile.FullName = strings.TrimSpace(parts[0])
		}
	}

	if err := chromedp.Run(browserCtx, chromedp.Evaluate(extractScript, &rawItems)); err != nil {
		return nil, fmt.Errorf("extracting posts for @%s: %w", profile.Handle, err)
	}

	allRawMap := make(map[string]RawDOMPost)
	orderedShortcodes := make([]string, 0)
	addRawItems := func(items []RawDOMPost) {
		for _, item := range items {
			shortcode := extractShortcode(item.HRef)
			if shortcode != "" {
				if _, exists := allRawMap[shortcode]; !exists {
					allRawMap[shortcode] = item
					orderedShortcodes = append(orderedShortcodes, shortcode)
				}
			}
		}
	}

	addRawItems(rawItems)
	log.Printf("[Browser] Initial grid loaded with %d posts for @%s", len(orderedShortcodes), profile.Handle)

	// Infinite scrolling loop: Instagram initial grid shows 12 items.
	// Clicking "Show more posts" expands to ~36 items, and scrolling down
	// loads subsequent batches into the DOM. Because Instagram virtualizes
	// (unmounts earlier items) during deep scrolling, we accumulate into
	// allRawMap after each scroll pass.
	scrollScript := `
		(() => {
			const buttons = Array.from(document.querySelectorAll('div[role="button"]'));
			const btn = buttons.find(b => b.innerText && b.innerText.includes('Show more posts'));
			if (btn) btn.click();

			window.scrollTo(0, document.body.scrollHeight || document.documentElement.scrollHeight);
			const sv = document.getElementById('scrollview') || document.querySelector('div[id="scrollview"]');
			if (sv) sv.scrollTop = sv.scrollHeight;
			const anchors = document.querySelectorAll('a[href*="/p/"], a[href*="/reel/"]');
			if (anchors.length > 0) anchors[anchors.length - 1].scrollIntoView();
			window.dispatchEvent(new Event('scroll'));
		})()
	`

	const maxScrollIterations = 5
	consecutiveNoNewCount := 0

	for iter := 1; iter <= maxScrollIterations; iter++ {
		// If sinceID is set and we've already collected it, stop scrolling early for fast delta sync
		if sinceID != "" && allRawMap[sinceID].HRef != "" {
			log.Printf("[Browser] Encountered previous latest post %s for @%s; stopping scroll early", sinceID, profile.Handle)
			break
		}

		prevCount := len(orderedShortcodes)

		// 1. Trigger scroll action
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(scrollScript, nil)); err != nil {
			log.Printf("[Browser] Warning on scroll iteration %d for @%s: %v", iter, profile.Handle, err)
			break
		}

		// 2. Wait dynamically for new items to load into the DOM (poll every 500ms up to 5s)
		scrollWaitCtx, scrollWaitCancel := context.WithTimeout(browserCtx, 5*time.Second)
		var scrolledItems []RawDOMPost

		for {
			select {
			case <-scrollWaitCtx.Done():
				goto ScrollWaitDone
			default:
			}

			time.Sleep(500 * time.Millisecond)
			scrolledItems = nil
			if err := chromedp.Run(browserCtx, chromedp.Evaluate(extractScript, &scrolledItems)); err != nil {
				break
			}

			// Check if any new shortcodes were discovered
			hasNew := false
			for _, item := range scrolledItems {
				sc := extractShortcode(item.HRef)
				if sc != "" && allRawMap[sc].HRef == "" {
					hasNew = true
					break
				}
			}

			if hasNew {
				// New content has loaded!
				break
			}
		}
	ScrollWaitDone:
		scrollWaitCancel()

		addRawItems(scrolledItems)
		newFound := len(orderedShortcodes) - prevCount
		log.Printf("[Browser] Scroll iteration %d for @%s: found %d new items (total unique: %d)", iter, profile.Handle, newFound, len(orderedShortcodes))

		if newFound == 0 {
			consecutiveNoNewCount++
			if consecutiveNoNewCount >= 2 {
				break
			}
		} else {
			consecutiveNoNewCount = 0
		}
	}

	posts := make([]model.Post, 0, len(orderedShortcodes))

	for _, sc := range orderedShortcodes {
		item := allRawMap[sc]
		if sinceID != "" && sc == sinceID {
			break
		}

		postURL := fmt.Sprintf("https://www.instagram.com/p/%s/", sc)
		isVideo := strings.Contains(item.HRef, "/reel")
		if isVideo {
			postURL = fmt.Sprintf("https://www.instagram.com/reel/%s/", sc)
		}

		pubTime := parseCaptionDate(item.Caption)
		if pubTime.IsZero() {
			pubTime = time.Now()
		}

		isCollab := len(sc) > 11
		trimmed := strings.Trim(item.HRef, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) >= 3 && (parts[1] == "reel" || parts[1] == "reels" || parts[1] == "p") {
			if profile.Handle != "" && !strings.EqualFold(parts[0], profile.Handle) {
				isCollab = true
			}
		}

		posts = append(posts, model.Post{
			ID:           sc,
			URL:          postURL,
			Caption:      item.Caption,
			ThumbnailURL: item.ImgURL,
			PublishedAt:  pubTime,
			Author:       profile.Handle,
			IsVideo:      isVideo,
			IsCollab:     isCollab,
		})
	}

	log.Printf("[Browser] Extracted total %d posts for @%s", len(posts), profile.Handle)
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
