package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/armbian/ansi-hastebin/config"
	"github.com/armbian/ansi-hastebin/handler"
	"github.com/armbian/ansi-hastebin/internal/keygenerator"
	"github.com/armbian/ansi-hastebin/internal/storage"
	"github.com/armbian/ansi-hastebin/static"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type Server struct {
	config       *config.Config
	storage      storage.Storage
	keyGenerator keygenerator.KeyGenerator
	server       *http.Server
	mux          *chi.Mux
	cleanupStop  chan struct{}
}

func NewServer(config *config.Config, storage storage.Storage, keyGenerator keygenerator.KeyGenerator) *Server {
	mux := chi.NewRouter()
	httpServer := &http.Server{
		Addr:              config.Host + ":" + strconv.Itoa(config.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return &Server{
		config:       config,
		storage:      storage,
		keyGenerator: keyGenerator,
		server:       httpServer,
		mux:          mux,
		cleanupStop:  make(chan struct{}),
	}
}

func (s *Server) RegisterRoutes() {
	// Register middlewares
	s.mux.Use(middleware.Logger)
	s.mux.Use(middleware.Recoverer)
	s.mux.Use(securityHeaders)

	// Rate limiter
	if s.config.RateLimiting.Enable {
		if s.config.RateLimiting.TrustedProxyCount > 0 {
			s.mux.Use(middleware.ClientIPFromXFFTrustedProxies(s.config.RateLimiting.TrustedProxyCount))
		} else {
			s.mux.Use(middleware.ClientIPFromRemoteAddr)
		}
		s.mux.Use(httprate.LimitBy(s.config.RateLimiting.Limit, time.Duration(s.config.RateLimiting.Window)*time.Second, func(r *http.Request) (string, error) {
			return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
		}))
	}

	// Register promhttp middleware
	s.mux.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Register document handler
	documentHandler := handler.NewDocumentHandler(s.config.KeyLength, s.config.MaxLength, s.storage, s.keyGenerator)
	documentHandler.DeleteAfterEnabled = s.config.DeleteAfter.Enable
	documentHandler.RegisterRoutes(s.mux)

	s.mux.Get("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			DeleteAfterEnabled bool `json:"delete_after_enabled"`
		}{DeleteAfterEnabled: s.config.DeleteAfter.Enable})
	})

	// Register health check
	s.mux.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Register static files
	fileServer := http.FileServer(http.FS(static.StaticFS))

	s.mux.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := static.StaticFS.Open(path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// If file does not exist, serve index.html
		index, err := static.StaticFS.Open("index.html")
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		defer index.Close()

		if _, err := io.Copy(w, index); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() {
	log.Info().Str("host", s.config.Host).Int("port", s.config.Port).Msg("Starting server")
	if cleaner, ok := s.storage.(storage.ExpiredPasteCleaner); ok {
		go s.cleanupLoop(cleaner)
	}

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

func (s *Server) cleanupLoop(cleaner storage.ExpiredPasteCleaner) {
	cleanup := func() {
		removed, err := cleaner.CleanupExpired()
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean expired pastes")
			return
		}
		if removed > 0 {
			log.Info().Int("removed", removed).Msg("Cleaned expired pastes")
		}
	}
	cleanup()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cleanup()
		case <-s.cleanupStop:
			return
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	log.Info().Msg("Gracefully shutting down server")
	close(s.cleanupStop)

	if err := s.storage.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close storage")
	}

	if err := s.server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to shutdown server")
	}
}
