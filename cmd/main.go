package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/armbian/ansi-hastebin/config"
	"github.com/armbian/ansi-hastebin/handler"
	"github.com/armbian/ansi-hastebin/keygenerator"
	"github.com/armbian/ansi-hastebin/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func main() {
	// Creater router instance
	r := chi.NewRouter()

	// Add several middlewares
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Check if config argument sent
	var configLocation string
	flag.StringVar(&configLocation, "config", "", "Pass config yaml")
	flag.Parse()

	// Parse config fields
	cfg := config.NewConfig(configLocation)

	var pasteStorage storage.Storage
	switch cfg.Storage.Type {
	case "file":
		pasteStorage = storage.NewFileStorage(cfg.Storage.FilePath, cfg.Expiration)
	case "redis":
		pasteStorage = storage.NewRedisStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Expiration)
	case "memcached":
		pasteStorage = storage.NewMemcachedStorage(cfg.Storage.Host, cfg.Storage.Port, int(cfg.Expiration))
	case "mongodb":
		pasteStorage = storage.NewMongoDBStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.Database, cfg.Expiration)
	case "postgres":
		pasteStorage = storage.NewPostgresStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.Database, int(cfg.Expiration))
	case "s3":
		pasteStorage = storage.NewS3Storage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.AWSRegion, cfg.Storage.Bucket)
	default:
		logrus.Fatalf("Unknown storage type: %s", cfg.Storage.Type)
		return
	}

	// Set static documents from config
	for _, doc := range cfg.Documents {
		file, err := os.OpenFile(doc.Path, os.O_RDONLY, 0644)
		if err != nil {
			logrus.WithError(err).WithField("path", doc.Path).Fatal("Failed to open document")
		}

		content, err := io.ReadAll(file)
		if err != nil {
			logrus.WithError(err).WithField("path", doc.Path).Fatal("Failed to read document")
		}

		file.Close()

		pasteStorage.Set(doc.Key, string(content), false)
	}

	var keyGenerator keygenerator.KeyGenerator

	switch cfg.KeyGenerator {
	case "random":
		keyGenerator = keygenerator.NewRandomKeyGenerator(cfg.KeySpace)
	case "phonetic":
		keyGenerator = keygenerator.NewPhoneticKeyGenerator()
	default:
		logrus.Fatalf("Unknown key generator: %s", cfg.KeyGenerator)
		return
	}

	// Add document handler
	document_handler := handler.NewDocumentHandler(cfg.KeyLength, cfg.MaxLength, pasteStorage, keyGenerator)

	// Add prometheus metrics
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Add document routes
	r.Get("/raw/{id}", document_handler.HandleRawGet)
	r.Head("/raw/{id}", document_handler.HandleRawGet)

	r.Post("/log", document_handler.HandlePutLog)
	r.Put("/log", document_handler.HandlePutLog)

	r.Post("/documents", document_handler.HandlePost)

	r.Get("/documents/{id}", document_handler.HandleGet)
	r.Head("/documents/{id}", document_handler.HandleGet)

	static := os.DirFS("static")
	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(cfg.Host+":"+strconv.Itoa(cfg.Port), r); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		return
	}
}
