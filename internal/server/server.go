package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/armbian/ansi-hastebin/config"
	"github.com/armbian/ansi-hastebin/handler"
	"github.com/armbian/ansi-hastebin/internal/keygenerator"
	"github.com/armbian/ansi-hastebin/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
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
	static := os.DirFS("static")
	s.mux.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if file, err := static.Open(id); err == nil {
			defer file.Close()
			io.Copy(w, file)
			return
		}

		index, err := static.Open("index.html")
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		defer index.Close()

		io.Copy(w, index)
	})

	fileServer := http.StripPrefix("/", http.FileServer(http.FS(static)))
	s.mux.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) Start() {
	logrus.Infof("Starting server on %s", s.server.Addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	logrus.Info("Gracefully shutting down server")

	if err := s.storage.Close(); err != nil {
		logrus.WithError(err).Error("Failed to close storage")
	}

	if err := s.server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown server")
	}
}
