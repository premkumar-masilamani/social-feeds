package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/engine"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

// Handler provides HTTP handlers for the application.
type Handler struct {
	engine  *engine.SyncEngine
	storage *feed.Storage
	baseURL string
}

// NewHandler creates a new Handler.
func NewHandler(eng *engine.SyncEngine, storage *feed.Storage, baseURL string) *Handler {
	return &Handler{
		engine:  eng,
		storage: storage,
		baseURL: baseURL,
	}
}

// HandleIndex renders the home page with feed lists and stats.
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	stats, err := h.storage.GetFeedStats(h.baseURL)
	if err != nil {
		http.Error(w, "Failed to load feed stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Group stats by platform
	grouped := make(map[string][]model.FeedStats)
	totalItems := 0
	for _, s := range stats {
		grouped[s.Platform] = append(grouped[s.Platform], s)
		totalItems += s.AllItemCount
	}

	isSyncing, lastSync, lastErr := h.engine.Status()

	data := DashboardData{
		BaseURL:      h.baseURL,
		Platforms:    grouped,
		IsSyncing:    isSyncing,
		LastSyncTime: lastSync,
		LastError:    lastErr,
		TotalFeeds:   len(stats),
		TotalItems:   totalItems,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := RenderDashboard(w, data); err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

// HandleFeed serves the Atom XML feed file with proper headers.
func (h *Handler) HandleFeed(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	filename := r.PathValue("filename")

	// Security: prevent path traversal attacks
	if strings.Contains(platform, "..") || strings.Contains(platform, "/") || strings.Contains(platform, "\\") {
		http.Error(w, "Invalid platform parameter", http.StatusBadRequest)
		return
	}
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename parameter", http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(filename, ".xml") {
		http.Error(w, "Only .xml feed files are served", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.storage.BaseDir(), platform, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Error reading feed file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(data)
}

// HandleTriggerSync starts an asynchronous sync run.
func (h *Handler) HandleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	started := h.engine.TriggerSync(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "syncing",
		"started": started,
	})
}

// HandleStatus returns the current sync status as JSON.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	isSyncing, lastSync, lastErr := h.engine.Status()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"is_syncing":     isSyncing,
		"last_sync_time": lastSync,
		"last_error":     lastErr,
	})
}

// HandleHealthz responds with health status.
func (h *Handler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
