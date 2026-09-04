package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/premkumar-masilamani/social-media-rss-feed/internal/engine"
	"github.com/premkumar-masilamani/social-media-rss-feed/internal/feed"
)

// Server encapsulates the HTTP server.
type Server struct {
	httpServer *http.Server
	port       int
}

// NewServer initializes the HTTP server with configured routes.
func NewServer(port int, baseURL string, eng *engine.SyncEngine, storage *feed.Storage) *Server {
	h := NewHandler(eng, storage, baseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.HandleIndex)
	mux.HandleFunc("GET /feeds/{platform}/{filename}", h.HandleFeed)
	mux.HandleFunc("POST /api/sync", h.HandleTriggerSync)
	mux.HandleFunc("GET /api/status", h.HandleStatus)
	mux.HandleFunc("GET /healthz", h.HandleHealthz)

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		httpServer: srv,
		port:       port,
	}
}

// Start runs the HTTP server listener.
func (s *Server) Start() error {
	log.Printf("[Server] Starting web server on http://localhost:%d", s.port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
