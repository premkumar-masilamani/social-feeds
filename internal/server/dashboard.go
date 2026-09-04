package server

import (
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/model"
)

// SyncNotice represents a user-friendly error explanation and actionable remedy.
type SyncNotice struct {
	Title    string
	Message  string
	Remedy   string
	Severity string // "warning" or "error"
	RawError string
}

// DashboardData holds view models for rendering the index page.
type DashboardData struct {
	BaseURL      string
	Platforms    map[string][]model.FeedStats
	IsSyncing    bool
	LastSyncTime time.Time
	LastError    string
	Notice       *SyncNotice
	TotalFeeds   int
	TotalItems   int
}

// ParseSyncNotice converts technical error logs into clean, human-friendly explanations and remedies.
func ParseSyncNotice(rawErr string) *SyncNotice {
	if rawErr == "" {
		return nil
	}

	lower := strings.ToLower(rawErr)

	if strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") {
		return &SyncNotice{
			Title:    "Instagram Rate Limit Active (HTTP 429)",
			Message:  "Instagram is temporarily throttling requests for one or more profiles because too many requests were made in a short time.",
			Remedy:   "Wait 5–10 minutes without clicking \"Sync Now\" to let Instagram's temporary cooldown counter reset. Your existing feeds are completely safe and still available.",
			Severity: "warning",
			RawError: rawErr,
		}
	}

	if strings.Contains(lower, "login failed") || strings.Contains(lower, "login wall") || strings.Contains(lower, "401") || strings.Contains(lower, "login required") {
		return &SyncNotice{
			Title:    "Instagram Login Required",
			Message:  "Instagram requires an active logged-in session to access profile posts.",
			Remedy:   "Ensure you have an active Instagram session in your browser profile (.browser_profile), or log in via browser.",
			Severity: "error",
			RawError: rawErr,
		}
	}

	if strings.Contains(lower, "private") {
		return &SyncNotice{
			Title:    "Private Profile Detected",
			Message:  "One of the requested profiles is set to Private on Instagram.",
			Remedy:   "Remove private handles from instagram.txt. Only public accounts can be syndicated into RSS feeds.",
			Severity: "warning",
			RawError: rawErr,
		}
	}

	if strings.Contains(lower, "not found") {
		return &SyncNotice{
			Title:    "Profile Not Found",
			Message:  "The requested username could not be found on Instagram.",
			Remedy:   "Check instagram.txt to ensure the username or URL is spelled correctly.",
			Severity: "warning",
			RawError: rawErr,
		}
	}

	return &SyncNotice{
		Title:    "Sync Notice",
		Message:  "Some profiles could not be updated during the latest sync pass. Existing feeds were preserved.",
		Remedy:   "Check your network connection and verify your input handles in the text file.",
		Severity: "warning",
		RawError: rawErr,
	}
}

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Social Media RSS Feeds</title>
  <style>
    :root {
      --bg: #0f172a;
      --card-bg: #1e293b;
      --border: #334155;
      --text: #f8fafc;
      --text-muted: #94a3b8;
      --accent: #38bdf8;
      --accent-hover: #0ea5e9;
      --badge-recent: #0284c7;
      --badge-all: #7c3aed;
      --success: #10b981;
      --warning: #f59e0b;
      --error: #ef4444;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    body { background-color: var(--bg); color: var(--text); padding: 2rem 1rem; line-height: 1.5; }
    .container { max-width: 1100px; margin: 0 auto; }
    header { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 1rem; margin-bottom: 2rem; padding-bottom: 1.5rem; border-bottom: 1px solid var(--border); }
    h1 { font-size: 1.8rem; font-weight: 700; color: #fff; display: flex; align-items: center; gap: 0.6rem; }
    .sync-btn {
      background: var(--accent);
      color: #0f172a;
      border: none;
      padding: 0.65rem 1.3rem;
      border-radius: 8px;
      font-weight: 600;
      font-size: 0.95rem;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      transition: background 0.15s ease, transform 0.05s ease;
    }
    .sync-btn:hover { background: var(--accent-hover); }
    .sync-btn:active { transform: scale(0.98); }
    .sync-btn:disabled { opacity: 0.6; cursor: not-allowed; }
    .status-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 1.5rem;
      background: var(--card-bg);
      padding: 1rem 1.25rem;
      border-radius: 10px;
      border: 1px solid var(--border);
      margin-bottom: 2rem;
      font-size: 0.9rem;
      color: var(--text-muted);
    }
    .status-item strong { color: var(--text); }
    .notice-card {
      border-radius: 10px;
      padding: 1.25rem 1.5rem;
      margin-bottom: 2rem;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.2);
    }
    .notice-warning {
      background: rgba(245, 158, 11, 0.08);
      border: 1px solid #f59e0b;
    }
    .notice-error {
      background: rgba(239, 68, 68, 0.08);
      border: 1px solid #ef4444;
    }
    .notice-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 0.5rem;
    }
    .notice-title {
      font-size: 1.1rem;
      font-weight: 700;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .notice-warning .notice-title { color: #fbbf24; }
    .notice-error .notice-title { color: #fca5a5; }
    .notice-badge {
      font-size: 0.75rem;
      text-transform: uppercase;
      padding: 0.2rem 0.6rem;
      border-radius: 4px;
      font-weight: 700;
      letter-spacing: 0.05em;
    }
    .notice-warning .notice-badge {
      background: rgba(245, 158, 11, 0.2);
      color: #fbbf24;
    }
    .notice-error .notice-badge {
      background: rgba(239, 68, 68, 0.2);
      color: #fca5a5;
    }
    .notice-message {
      color: var(--text);
      font-size: 0.95rem;
      margin-bottom: 0.85rem;
      line-height: 1.4;
    }
    .notice-remedy-box {
      background: rgba(15, 23, 42, 0.65);
      border-left: 4px solid var(--accent);
      border-radius: 6px;
      padding: 0.85rem 1.1rem;
      margin-bottom: 0.75rem;
    }
    .notice-remedy-label {
      font-size: 0.85rem;
      font-weight: 700;
      color: var(--accent);
      margin-bottom: 0.25rem;
      display: flex;
      align-items: center;
      gap: 0.4rem;
    }
    .notice-remedy-text {
      font-size: 0.92rem;
      color: #e2e8f0;
      line-height: 1.45;
    }
    .notice-details {
      font-size: 0.8rem;
      color: var(--text-muted);
      cursor: pointer;
      margin-top: 0.5rem;
    }
    .notice-details summary:hover {
      color: var(--text);
    }
    .notice-details code {
      display: block;
      margin-top: 0.4rem;
      padding: 0.5rem 0.75rem;
      background: #0f172a;
      border-radius: 6px;
      word-break: break-all;
      color: #94a3b8;
      font-size: 0.78rem;
    }
    .platform-section { margin-bottom: 2.5rem; }
    .platform-title {
      font-size: 1.3rem;
      text-transform: capitalize;
      margin-bottom: 1rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
      color: var(--accent);
    }
    .cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 1.25rem; }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 1.25rem;
      display: flex;
      flex-direction: column;
      gap: 1rem;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.2);
    }
    .card-header { display: flex; justify-content: space-between; align-items: center; }
    .profile-handle { font-size: 1.15rem; font-weight: 700; color: #fff; text-decoration: none; }
    .profile-handle:hover { color: var(--accent); text-decoration: underline; }
    .profile-badge { font-size: 0.75rem; padding: 0.2rem 0.5rem; border-radius: 6px; background: rgba(56, 189, 248, 0.15); color: var(--accent); font-weight: 600; text-transform: uppercase; }
    .feed-links { display: flex; flex-direction: column; gap: 0.75rem; }
    .feed-box {
      background: #0f172a;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 0.75rem;
      display: flex;
      flex-direction: column;
      gap: 0.4rem;
    }
    .feed-box-header { display: flex; justify-content: space-between; align-items: center; font-size: 0.85rem; }
    .badge {
      font-size: 0.75rem;
      padding: 0.15rem 0.5rem;
      border-radius: 4px;
      font-weight: 600;
      color: #fff;
    }
    .badge-recent { background: var(--badge-recent); }
    .badge-all { background: var(--badge-all); }
    .feed-stats { font-size: 0.8rem; color: var(--text-muted); }
    .feed-url-row { display: flex; gap: 0.4rem; align-items: center; margin-top: 0.2rem; }
    .feed-url {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.8rem;
      color: var(--accent);
      text-decoration: none;
      word-break: break-all;
      flex: 1;
    }
    .feed-url:hover { text-decoration: underline; }
    .copy-btn {
      background: #334155;
      color: var(--text);
      border: none;
      border-radius: 4px;
      padding: 0.25rem 0.5rem;
      font-size: 0.75rem;
      cursor: pointer;
      white-space: nowrap;
    }
    .copy-btn:hover { background: #475569; }
    .empty-state { text-align: center; padding: 3rem; background: var(--card-bg); border-radius: 12px; border: 1px solid var(--border); color: var(--text-muted); }
    .empty-state code { background: #0f172a; padding: 0.2rem 0.4rem; border-radius: 4px; color: var(--accent); }
    .spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid rgba(15, 23, 42, 0.25);
      border-radius: 50%;
      border-top-color: #0f172a;
      animation: spin 0.8s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <h1>📡 Social Media RSS Feeds</h1>
        <p style="color: var(--text-muted); font-size: 0.95rem; margin-top: 0.2rem;">Local Atom 1.0 XML feeds for public social media profiles</p>
      </div>
      <div>
        <button id="sync-btn" class="sync-btn" onclick="triggerSync()">
          <span id="sync-icon">🔄</span>
          <span id="sync-text">Sync Now</span>
        </button>
      </div>
    </header>

    {{if .Notice}}
    <div class="notice-card notice-{{.Notice.Severity}}">
      <div class="notice-header">
        <div class="notice-title">
          {{if eq .Notice.Severity "warning"}}⏳{{else}}🔑{{end}} {{.Notice.Title}}
        </div>
        <span class="notice-badge">{{if eq .Notice.Severity "warning"}}Temporary Notice{{else}}Action Needed{{end}}</span>
      </div>
      <p class="notice-message">{{.Notice.Message}}</p>
      <div class="notice-remedy-box">
        <div class="notice-remedy-label">💡 Recommended Remedy:</div>
        <div class="notice-remedy-text">{{.Notice.Remedy}}</div>
      </div>
      <details class="notice-details">
        <summary>View technical log</summary>
        <code>{{.Notice.RawError}}</code>
      </details>
    </div>
    {{end}}

    <div class="status-bar">
      <div class="status-item">Total Profiles: <strong>{{.TotalFeeds}}</strong></div>
      <div class="status-item">Total Cached Posts: <strong>{{.TotalItems}}</strong></div>
      <div class="status-item">Last Sync: <strong>{{if .LastSyncTime.IsZero}}Never{{else}}{{.LastSyncTime.Format "15:04:05 UTC (Jan 02)"}}{{end}}</strong></div>
      <div class="status-item">Sync Status: <strong id="status-badge" style="color: {{if .IsSyncing}}var(--warning){{else}}var(--success){{end}};">{{if .IsSyncing}}Syncing in background...{{else}}Idle / Ready{{end}}</strong></div>
    </div>

    {{if eq .TotalFeeds 0}}
    <div class="empty-state">
      <h2>No Feeds Generated Yet</h2>
      <p style="margin-top: 0.75rem;">Add handles to <code>instagram.txt</code> (one per line) and click <strong>Sync Now</strong>.</p>
    </div>
    {{else}}
      {{range $platform, $feeds := .Platforms}}
      <section class="platform-section">
        <h2 class="platform-title">📂 {{$platform}} Feeds</h2>
        <div class="cards-grid">
          {{range $feeds}}
          <div class="card">
            <div class="card-header">
              <a href="{{.ProfileURL}}" target="_blank" rel="noopener noreferrer" class="profile-handle">@{{.Handle}}</a>
              <span class="profile-badge">{{.Platform}}</span>
            </div>

            <div class="feed-links">
              <!-- Recent Feed (Capped at RecentFeedLimit) -->
              <div class="feed-box">
                <div class="feed-box-header">
                  <span class="badge badge-recent">Recent Feed (15 Posts)</span>
                  <span class="feed-stats">{{.RecentItemCount}} items &bull; {{.RecentFileSizeBytes}} B</span>
                </div>
                <div class="feed-url-row">
                  <a href="{{.RecentFeedURL}}" class="feed-url" target="_blank">{{.Handle}}-feed.xml</a>
                  <button class="copy-btn" onclick="copyToClipboard('{{.RecentFeedURL}}', this)">Copy URL</button>
                </div>
              </div>

              <!-- Full Archive Feed -->
              <div class="feed-box">
                <div class="feed-box-header">
                  <span class="badge badge-all">Full Archive</span>
                  <span class="feed-stats">{{.AllItemCount}} items &bull; {{.AllFileSizeBytes}} B</span>
                </div>
                <div class="feed-url-row">
                  <a href="{{.AllFeedURL}}" class="feed-url" target="_blank">{{.Handle}}-all-feed.xml</a>
                  <button class="copy-btn" onclick="copyToClipboard('{{.AllFeedURL}}', this)">Copy URL</button>
                </div>
              </div>
            </div>

            <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: auto;">
              Last Updated: {{if .LastUpdated.IsZero}}Never{{else}}{{.LastUpdated.Format "2006-01-02 15:04:05"}}{{end}}
            </div>
          </div>
          {{end}}
        </div>
      </section>
      {{end}}
    {{end}}
  </div>

  <script>
    async function triggerSync() {
      const btn = document.getElementById('sync-btn');
      const icon = document.getElementById('sync-icon');
      const text = document.getElementById('sync-text');
      const badge = document.getElementById('status-badge');

      btn.disabled = true;
      icon.innerHTML = '<span class="spinner"></span>';
      text.textContent = 'Syncing...';
      badge.textContent = 'Syncing in background...';
      badge.style.color = 'var(--warning)';

      try {
        const res = await fetch('/api/sync', { method: 'POST' });
        if (res.ok) {
          pollSyncStatus();
        } else {
          alert('Sync trigger failed');
          resetBtn();
        }
      } catch (err) {
        alert('Sync error: ' + err.message);
        resetBtn();
      }
    }

    async function pollSyncStatus() {
      const check = async () => {
        try {
          const res = await fetch('/api/status');
          const data = await res.json();
          if (!data.is_syncing) {
            window.location.reload();
          } else {
            setTimeout(check, 1500);
          }
        } catch {
          setTimeout(check, 2500);
        }
      };
      setTimeout(check, 1500);
    }

    function resetBtn() {
      const btn = document.getElementById('sync-btn');
      const icon = document.getElementById('sync-icon');
      const text = document.getElementById('sync-text');
      btn.disabled = false;
      icon.textContent = '🔄';
      text.textContent = 'Sync Now';
    }

    function copyToClipboard(text, btn) {
      navigator.clipboard.writeText(text).then(() => {
        const old = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => btn.textContent = old, 1500);
      });
    }
  </script>
</body>
</html>`

var parsedTemplate = template.Must(template.New("index").Parse(indexTemplate))

// RenderDashboard renders the HTML dashboard to w.
func RenderDashboard(w io.Writer, data DashboardData) error {
	return parsedTemplate.Execute(w, data)
}
