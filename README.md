# Social Media RSS Feed Generator (`social-rss`)

A fast, lightweight, and extensible CLI daemon written in Go that generates local **Atom 1.0 XML feeds** for public social media profiles (starting with Instagram).

It runs quietly in the background on your machine, maintains dual feeds per profile (latest 50 posts vs. full archive), performs idempotent delta syncs, and provides a web dashboard on port `9527` with a global **"Sync Now"** button.

---

## Features

- ⚡ **Zero Binary Media Downloads:** Only downloads metadata (captions, timestamps, URLs, author) and embeds remote CDN thumbnail images (`<img src="...">`) directly inside feed items so your RSS reader displays pictures without bloating your hard drive.
- 🔄 **Dual Atom Feeds Per Profile:**
  - `<handle>-feed.xml`: Latest 50 posts (fast reader parsing, minimal overhead).
- 🛡️ **Headless Chrome Automation:** Uses automated headless Chrome via Chrome DevTools Protocol (`chromedp`) with automatic login (`INSTAGRAM_USERNAME` and `INSTAGRAM_PASSWORD`) to navigate Instagram exactly like a browser user, bypassing raw HTTP rate-limiting and login blocks.
- 🔁 **Idempotent Delta Syncing:** Uses the XML feeds on disk as the state store to detect known posts, merge new content, and prevent duplicates.
- 🖥️ **Web Dashboard & "Sync Now":** Embedded web dashboard on `http://localhost:9527` displaying profiles, feed URLs, item counts, file sizes, and an on-demand "Sync Now" button.
- 🔌 **Extensible Architecture:** Designed with a `PlatformProvider` interface. Auto-detects input files (e.g., `instagram.txt`, future `facebook.txt`, `x.txt`).

---

## Quick Start

### 1. Build & Run with Make

```bash
# Build the binary into ./bin/social-rss
make build

# Run unit tests with race detection
make test

# Run code linter and formatting checks
make lint

# Run the app
make run
```

### 2. Configure Profiles

Add profile URLs or usernames to `instagram.txt` (one per line). Supported formats:

```text
# Examples of supported formats:
https://www.instagram.com/natgeo/
https://instagram.com/nasa
@cristiano
leomessi
```

### 3. Open Web Dashboard

Navigate to:
```
http://localhost:9527
```
Subscribe to any feed link in your favorite RSS reader (e.g. NetNewsWire, Reeder, Feedly, FreshRSS):
- Recent 50: `http://localhost:9527/feeds/instagram/natgeo-feed.xml`
- Full Archive: `http://localhost:9527/feeds/instagram/natgeo-all-feed.xml`

---

## Makefile Targets

| Target | Description |
| :--- | :--- |
| `make build` | Compiles the binary to `./bin/social-rss` |
| `make run` | Builds and starts the application |
| `make test` | Runs the full test suite with `-race` detection |
| `make lint` | Checks code formatting (`gofmt`) and runs `go vet` |
| `make fmt` | Formats all Go files with `gofmt` |
| `make clean` | Removes compiled binaries and test artifacts |
| `make help` | Displays list of available Makefile targets |

---

## Configuration

The default HTTP port is set to **`9527`** (avoiding standard software development ports like 3000, 5000, 8080, or 8000) so it can run persistently in the background.

| Setting | Flag | Environment Variable | Default |
| :--- | :--- | :--- | :--- |
| HTTP Port | `-port <int>` | `PORT` | `9527` |
| Poll Frequency | `-poll <duration>` | - | `1h` (e.g., `5m`, `30m`, `24h`) |
| Feeds Directory | `-feeds-dir <path>` | `FEEDS_DIR` | `./feeds` |
| Base URL | `-base-url <url>` | `BASE_URL` | `http://localhost:9527` |
| Instagram Username | - | `INSTAGRAM_USERNAME` | Username in `.env` |
| Instagram Password | - | `INSTAGRAM_PASSWORD` | Password in `.env` |

---

## Running in the Background (OS Autostart / Daemon Setup)

To keep `social-rss` running quietly in the background on your machine and start automatically at boot, follow the instructions for your operating system:

### 1. macOS (`launchd`)

macOS uses `launchd` for user-level background services:

1. Build the binary and place it in a permanent path (e.g., `/usr/local/bin` or your code directory):
   ```bash
   make build
   mkdir -p ~/bin
   cp bin/social-rss ~/bin/social-rss
   ```

2. Create a LaunchAgent plist file at `~/Library/LaunchAgents/com.user.social-rss.plist`:
   ```xml
   <?xml version="1.0" encoding="UTF-8"?>
   <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
   <plist version="1.0">
   <dict>
       <key>Label</key>
       <string>com.user.social-rss</string>
       <key>ProgramArguments</key>
       <array>
           <string>/Users/YOUR_USERNAME/bin/social-rss</string>
       </array>
       <key>WorkingDirectory</key>
       <string>/Users/YOUR_USERNAME/Code/social-media-rss-feed</string>
       <key>RunAtLoad</key>
       <true/>
       <key>KeepAlive</key>
       <true/>
       <key>StandardOutPath</key>
       <string>/tmp/social-rss.log</string>
       <key>StandardErrorPath</key>
       <string>/tmp/social-rss.err</string>
   </dict>
   </plist>
   ```
   *(Replace `YOUR_USERNAME` and the working directory with your actual paths).*

3. Load and start the service:
   ```bash
   launchctl load ~/Library/LaunchAgents/com.user.social-rss.plist
   ```
   To stop or unload:
   ```bash
   launchctl unload ~/Library/LaunchAgents/com.user.social-rss.plist
   ```

---

### 2. Linux (`systemd` User Service or `cron`)

#### Method A: `systemd` User Service (Recommended)

1. Copy the built binary to your local bin:
   ```bash
   make build
   mkdir -p ~/.local/bin
   cp bin/social-rss ~/.local/bin/social-rss
   ```

2. Create a systemd user unit file at `~/.config/systemd/user/social-rss.service`:
   ```ini
   [Unit]
   Description=Social Media RSS Feed Daemon
   After=network.target

   [Service]
   Type=simple
   WorkingDirectory=%h/Code/social-media-rss-feed
   ExecStart=%h/.local/bin/social-rss
   Restart=always
   RestartSec=10
   Environment=PORT=9527

   [Install]
   WantedBy=default.target
   ```

3. Enable and start the service:
   ```bash
   systemctl --user daemon-reload
   systemctl --user enable --now social-rss.service
   ```

4. Check logs:
   ```bash
   journalctl --user -u social-rss.service -f
   ```

#### Method B: Linux `crontab` (@reboot)

You can also start it at system startup via `cron`:
```bash
crontab -e
```
Add the line:
```text
@reboot cd /path/to/social-media-rss-feed && ./bin/social-rss >> /var/log/social-rss.log 2>&1 &
```

---

### 3. Windows (Task Scheduler or NSSM)

#### Method A: Task Scheduler GUI or CLI (`schtasks`)

1. Build for Windows:
   ```bash
   go build -o social-rss.exe ./cmd/social-rss
   ```

2. Register a startup task with Command Prompt / PowerShell (run as Administrator):
   ```cmd
   schtasks /create /tn "SocialMediaRSS" /tr "C:\path\to\social-media-rss-feed\social-rss.exe" /sc onlogon /rl highest
   ```

#### Method B: NSSM (Non-Sucking Service Manager)

To run as a native Windows Service in the background:
```cmd
nssm install SocialMediaRSS C:\path\to\social-media-rss-feed\social-rss.exe
nssm set SocialMediaRSS AppDirectory C:\path\to\social-media-rss-feed
nssm start SocialMediaRSS
```

---

## Multi-Platform Extensibility

The tool uses a pluggable `PlatformProvider` interface:

```go
type PlatformProvider interface {
    Name() string
    SourceFile() string
    ParseTarget(line string) (*model.Profile, error)
    FetchPosts(ctx context.Context, profile *model.Profile, sinceID string) ([]model.Post, error)
}
```

To add a new platform (such as Facebook or X):
1. Implement the `PlatformProvider` interface under `internal/provider/<platform>/`.
2. Register the provider in `init()` using `provider.Register(...)`.
3. Add a corresponding text file (e.g. `facebook.txt` or `x.txt`) in your working directory.
4. The application automatically discovers the file and begins generating feeds into `./feeds/<platform>/`!
