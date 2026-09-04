package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/engine"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

func setupTestServer(t *testing.T) (*Handler, *feed.Storage, string) {
	tempDir := t.TempDir()
	storage := feed.NewStorage(tempDir)
	baseURL := "http://localhost:9527"
	eng := engine.NewSyncEngine(tempDir, baseURL, storage)
	h := NewHandler(eng, storage, baseURL)
	return h, storage, tempDir
}

func TestHandleIndex(t *testing.T) {
	h, storage, _ := setupTestServer(t)

	// Seed one feed
	profile := &model.Profile{
		Platform: "instagram",
		Handle:   "testaccount",
		URL:      "https://www.instagram.com/testaccount/",
	}
	_ = storage.SavePosts("http://localhost:9527", profile, []model.Post{
		{ID: "1", URL: "https://www.instagram.com/p/1/", Caption: "Hello World", PublishedAt: time.Now()},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.HandleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !contains(body, "Social Media RSS Feeds") {
		t.Errorf("expected dashboard title in HTML output")
	}
	if !contains(body, "@testaccount") {
		t.Errorf("expected profile handle in dashboard HTML")
	}
	if !contains(body, "testaccount-feed.xml") {
		t.Errorf("expected feed links in dashboard HTML")
	}
}

func TestHandleFeed(t *testing.T) {
	h, storage, _ := setupTestServer(t)

	profile := &model.Profile{
		Platform: "instagram",
		Handle:   "photographer",
		URL:      "https://www.instagram.com/photographer/",
	}
	_ = storage.SavePosts("http://localhost:9527", profile, []model.Post{
		{ID: "1", URL: "https://www.instagram.com/p/1/", Caption: "Golden hour", PublishedAt: time.Now()},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /feeds/{platform}/{filename}", h.HandleFeed)

	req := httptest.NewRequest(http.MethodGet, "/feeds/instagram/photographer-feed.xml", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/atom+xml; charset=utf-8" {
		t.Errorf("unexpected content-type: %s", contentType)
	}

	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("unexpected cache-control: %s", cacheControl)
	}
}

func TestHandleFeedSecurity(t *testing.T) {
	h, _, _ := setupTestServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /feeds/{platform}/{filename}", h.HandleFeed)

	// Path traversal attempt in filename
	req := httptest.NewRequest(http.MethodGet, "/feeds/instagram/..%2fsecret.xml", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404 for traversal attempt, got %d", rr.Code)
	}

	// Non-xml extension
	req = httptest.NewRequest(http.MethodGet, "/feeds/instagram/config.json", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-xml extension, got %d", rr.Code)
	}
}

func TestHandleTriggerSync(t *testing.T) {
	h, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	rr := httptest.NewRecorder()

	h.HandleTriggerSync(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var res map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if res["status"] != "syncing" {
		t.Errorf("expected status 'syncing', got %v", res["status"])
	}
}

func TestHandleStatus(t *testing.T) {
	h, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()

	h.HandleStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var res map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode status JSON: %v", err)
	}

	if _, ok := res["is_syncing"]; !ok {
		t.Errorf("missing is_syncing in status response")
	}
}

func TestHandleHealthz(t *testing.T) {
	h, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.HandleHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestParseSyncNotice(t *testing.T) {
	// Empty string
	if ParseSyncNotice("") != nil {
		t.Errorf("expected nil notice for empty error")
	}

	// Rate limit 429
	notice429 := ParseSyncNotice("rate limited (HTTP 429) by Instagram when fetching @user")
	if notice429 == nil || notice429.Severity != "warning" || !contains(notice429.Title, "429") {
		t.Errorf("unexpected 429 notice: %+v", notice429)
	}
	if !contains(notice429.Remedy, "Wait 5–10 minutes") {
		t.Errorf("remedy should advise waiting: %s", notice429.Remedy)
	}

	// Login wall / login required
	notice401 := ParseSyncNotice("access denied (HTTP 401) by Instagram for @user (login wall)")
	if notice401 == nil || notice401.Severity != "error" || !contains(notice401.Title, "Login Required") {
		t.Errorf("unexpected 401 notice: %+v", notice401)
	}
	if !contains(notice401.Remedy, "INSTAGRAM_USERNAME") {
		t.Errorf("remedy should advise setting INSTAGRAM_USERNAME: %s", notice401.Remedy)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && s[0:len(substr)] == substr || (len(s) > 1 && contains(s[1:], substr))))
}
