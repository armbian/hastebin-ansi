package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/armbian/ansi-hastebin/config"
	"github.com/armbian/ansi-hastebin/internal/keygenerator"
	"github.com/armbian/ansi-hastebin/internal/server"
	"github.com/armbian/ansi-hastebin/internal/storage"
	"github.com/sirupsen/logrus"
)

func handleConfig(location string) (*config.Config, storage.Storage, keygenerator.KeyGenerator) {
	cfg := config.NewConfig(location)
	exp := time.Duration(cfg.Expiration)

	var pasteStorage storage.Storage
	switch cfg.Storage.Type {
	case "file":
		pasteStorage = storage.NewFileStorage(cfg.Storage.FilePath, exp)
	case "redis":
		pasteStorage = storage.NewRedisStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, exp)
	case "memcached":
		pasteStorage = storage.NewMemcachedStorage(cfg.Storage.Host, cfg.Storage.Port, int(cfg.Expiration))
	case "mongodb":
		pasteStorage = storage.NewMongoDBStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.Database, exp)
	case "postgres":
		pasteStorage = storage.NewPostgresStorage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.Database, int(cfg.Expiration))
	case "s3":
		pasteStorage = storage.NewS3Storage(cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.Username, cfg.Storage.Password, cfg.Storage.AWSRegion, cfg.Storage.Bucket)
	default:
		logrus.Fatalf("Unknown storage type: %s", cfg.Storage.Type)
		return nil, nil, nil
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

		if err := pasteStorage.Set(doc.Key, string(content), false); err != nil {
			logrus.WithError(err).WithField("key", doc.Key).Fatal("Failed to set document")
		}
	}

	var keyGenerator keygenerator.KeyGenerator

	switch cfg.KeyGenerator {
	case "random":
		keyGenerator = keygenerator.NewRandomKeyGenerator(cfg.KeySpace)
	case "phonetic":
		keyGenerator = keygenerator.NewPhoneticKeyGenerator()
	default:
		logrus.Fatalf("Unknown key generator: %s", cfg.KeyGenerator)
		return nil, nil, nil
	}

	return cfg, pasteStorage, keyGenerator
}

func main() {
	// Parse command line arguments
	var configFile string
	flag.StringVar(&configFile, "config", "config.yaml", "Configuration file")
	flag.Parse()

	srv := server.NewServer(handleConfig(configFile))
	srv.RegisterRoutes()

	// Start the server in a separate goroutine
	go func() {
		srv.Start()
	}()

	// Wait for signal to stop the server
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	<-stopCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
}
