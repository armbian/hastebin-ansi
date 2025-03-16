package server

import (
	"context"
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
}

func NewServer(config *config.Config, storage storage.Storage, keyGenerator keygenerator.KeyGenerator) *Server {
	mux := chi.NewRouter()
	httpServer := &http.Server{
		Addr:    config.Host + ":" + strconv.Itoa(config.Port),
		Handler: mux,
	}

	return &Server{
		config:       config,
		storage:      storage,
		keyGenerator: keyGenerator,
		server:       httpServer,
		mux:          mux,
	}
}

func (s *Server) RegisterRoutes() {
	// Register middlewares
	s.mux.Use(middleware.Logger)
	s.mux.Use(middleware.Recoverer)

	// Rate limiter
	if s.config.RateLimiting.Enable {
		s.mux.Use(httprate.LimitByRealIP(s.config.RateLimiting.Limit, time.Duration(s.config.RateLimiting.Window)*time.Second))
	}

	// Register promhttp middleware
	s.mux.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Register document handler
	documentHandler := handler.NewDocumentHandler(s.config.KeyLength, s.config.MaxLength, s.storage, s.keyGenerator)
	documentHandler.RegisterRoutes(s.mux)

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

func (s *Server) Start() {
	log.Info().Str("host", s.config.Host).Int("port", s.config.Port).Msg("Starting server")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	log.Info().Msg("Gracefully shutting down server")

	if err := s.storage.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close storage")
	}

	if err := s.server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to shutdown server")
	}
}
