package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type LoggingConfig struct {
	Level    string `yaml:"level"`
	Type     string `yaml:"type"`
	Colorize bool   `yaml:"colorize"`
}

type StorageConfig struct {
	// Type is the storage backend to use
	// Available storage backends are: "redis", "file", "memcached", "mongodb", "s3", "postgres"
	Type string `yaml:"type"`

	// Host is the hostname or IP address of the storage backend
	Host string `yaml:"host"`

	// Port is the port of the storage backend
	Port int `yaml:"port"`

	// Username is the username to use for the storage backend
	Username string `yaml:"username"`

	// Password is the password to use for the storage backend
	Password string `yaml:"password"`

	// Database is the database to use for the storage backend
	Database string `yaml:"database"`

	// Bucket is the bucket to use for the storage backend
	Bucket string `yaml:"bucket"`

	// AWSRegion is the AWS region to use for the storage backend
	// This property is only used for the "s3" storage backend
	AWSRegion string `yaml:"aws_region"`

	// FilePath is the file path to use for the "file" storage backend
	// This property is only used for the "file" storage backend
	FilePath string `yaml:"file_path"`
}

type DocumentConfig struct {
	// Key is the key of the document
	Key string `yaml:"key"`

	// Path is the path of the document which is going to be read
	Path string `yaml:"path"`
}

type Config struct {
	// Host is the hostname or IP address to bind to
	Host string `yaml:"host"`

	// Port is the port to bind to
	Port int `yaml:"port"`

	// KeyLength is the length of the key to generate which is used for storage key
	KeyLength int `yaml:"key_length"`

	// KeySpace is the key space to use for the key generator
	// This property is only used for the "random" key generator
	KeySpace string `yaml:"key_space"`

	// MaxLength is the maximum length of the paste
	MaxLength int `yaml:"max_length"`

	// StaticMaxAge is the maximum age of static assets
	StaticMaxAge int `yaml:"static_max_age"`

	// Expiration is the maximum lifetime of paste entry
	// 0 means there will be no expiration.
	// "file" and "s3" storages don't support expiration control.
	Expiration time.Duration `yaml:"expiration"`

	// RecompressStaticAssets is a flag to recompress static assets by default
	RecompressStaticAssets bool `yaml:"recompress_static_assets"`

	// KeyGenerator is the key generator to use
	// Available key generators are: "random", "phonetic"
	KeyGenerator string `yaml:"key_generator"`

	// Storage is the storage backend to use
	// Available storage backends are: "redis", "file", "memcached", "mongodb", "s3", "postgres"
	Storage StorageConfig `yaml:"storage"`

	// Logging is the logging configuration
	Logging LoggingConfig `yaml:"logging"`

	// Documents is the list of documents to load statically
	Documents []DocumentConfig `yaml:"documents"`
}

var DefaultConfig = &Config{
	Host:                   "0.0.0.0",
	Port:                   7777,
	KeyLength:              10,
	MaxLength:              4000000,
	StaticMaxAge:           3,
	RecompressStaticAssets: false,
	KeyGenerator:           "phonetic",
	Storage: StorageConfig{
		Type:     "file",
		FilePath: "data",
	},
	Logging: LoggingConfig{
		Level: "info",
		Type:  "text",
	},
	Documents: []DocumentConfig{
		{
			Key:  "about",
			Path: "about.md",
		},
	},
}

// NewConfig creates a new Config instance
func NewConfig(configFile string) *Config {
	cfg := &Config{}

	// Read the configuration file
	data, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		logrus.WithError(err).Fatal("Failed to read configuration file")
	}

	// Unmarshal the configuration file
	if err := yaml.Unmarshal(data, cfg); err != nil {
		logrus.WithError(err).Fatal("Failed to unmarshal configuration file")
	}

	// Override with environment variables
	if host := os.Getenv("HOST"); host != "" {
		cfg.Host = host
	}

	if port := os.Getenv("PORT"); port != "" {
		portInt, err := strconv.Atoi(port)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse PORT environment variable")
		}
		cfg.Port = portInt
	}

	if keyLength := os.Getenv("KEY_LENGTH"); keyLength != "" {
		keyLengthInt, err := strconv.Atoi(keyLength)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse KEY_LENGTH environment variable")
		}
		cfg.KeyLength = keyLengthInt
	}

	if maxLength := os.Getenv("MAX_LENGTH"); maxLength != "" {
		maxLengthInt, err := strconv.Atoi(maxLength)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse MAX_LENGTH environment variable")
		}
		cfg.MaxLength = maxLengthInt
	}

	if staticMaxAge := os.Getenv("STATIC_MAX_AGE"); staticMaxAge != "" {
		staticMaxAgeInt, err := strconv.Atoi(staticMaxAge)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse STATIC_MAX_AGE environment variable")
		}
		cfg.StaticMaxAge = staticMaxAgeInt
	}

	if recompressStaticAssets := os.Getenv("RECOMPRESS_STATIC_ASSETS"); recompressStaticAssets != "" {
		recompressStaticAssetsBool, err := strconv.ParseBool(recompressStaticAssets)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse RECOMPRESS_STATIC_ASSETS environment variable")
		}
		cfg.RecompressStaticAssets = recompressStaticAssetsBool
	}

	if keyGenerator := os.Getenv("KEY_GENERATOR"); keyGenerator != "" {
		cfg.KeyGenerator = keyGenerator
	}

	if storageType := os.Getenv("STORAGE_TYPE"); storageType != "" {
		cfg.Storage.Type = storageType
	}

	if storageHost := os.Getenv("STORAGE_HOST"); storageHost != "" {
		cfg.Storage.Host = storageHost
	}

	if storagePort := os.Getenv("STORAGE_PORT"); storagePort != "" {
		storagePortInt, err := strconv.Atoi(storagePort)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse STORAGE_PORT environment variable")
		}
		cfg.Storage.Port = storagePortInt
	}

	if storageUsername := os.Getenv("STORAGE_USERNAME"); storageUsername != "" {
		cfg.Storage.Username = storageUsername
	}

	if storagePassword := os.Getenv("STORAGE_PASSWORD"); storagePassword != "" {
		cfg.Storage.Password = storagePassword
	}

	if storageDatabase := os.Getenv("STORAGE_DATABASE"); storageDatabase != "" {
		cfg.Storage.Database = storageDatabase
	}

	if storageBucket := os.Getenv("STORAGE_BUCKET"); storageBucket != "" {
		cfg.Storage.Bucket = storageBucket
	}

	if storageAWSRegion := os.Getenv("STORAGE_AWS_REGION"); storageAWSRegion != "" {
		cfg.Storage.AWSRegion = storageAWSRegion
	}

	if storageFilePath := os.Getenv("STORAGE_FILE_PATH"); storageFilePath != "" {
		cfg.Storage.FilePath = storageFilePath
	}

	if loggingLevel := os.Getenv("LOGGING_LEVEL"); loggingLevel != "" {
		cfg.Logging.Level = loggingLevel
	}

	if loggingType := os.Getenv("LOGGING_TYPE"); loggingType != "" {
		cfg.Logging.Type = loggingType
	}

	if loggingColorize := os.Getenv("LOGGING_COLORIZE"); loggingColorize != "" {
		loggingColorizeBool, err := strconv.ParseBool(loggingColorize)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse LOGGING_COLORIZE environment variable")
		}
		cfg.Logging.Colorize = loggingColorizeBool
	}

	// Walk environment variables for documents
	for _, env := range os.Environ() {
		if len(env) > 10 && env[:10] == "DOCUMENTS_" {
			parts := strings.Split(env[10:], "=")
			if len(parts) == 2 {
				cfg.Documents = append(cfg.Documents, DocumentConfig{
					Key:  parts[0],
					Path: parts[1],
				})
			}
		}
	}

	// Apply default values to the configuration
	if cfg.Host == "" {
		cfg.Host = DefaultConfig.Host
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultConfig.Port
	}

	if cfg.KeyLength == 0 {
		cfg.KeyLength = DefaultConfig.KeyLength
	}

	if cfg.MaxLength == 0 {
		cfg.MaxLength = DefaultConfig.MaxLength
	}

	if cfg.StaticMaxAge == 0 {
		cfg.StaticMaxAge = DefaultConfig.StaticMaxAge
	}

	if cfg.KeyGenerator == "" {
		cfg.KeyGenerator = DefaultConfig.KeyGenerator
	}

	if cfg.Storage.Type == "" {
		cfg.Storage.Type = DefaultConfig.Storage.Type
	}

	if cfg.Storage.FilePath == "" {
		cfg.Storage.FilePath = DefaultConfig.Storage.FilePath
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = DefaultConfig.Logging.Level
	}

	if cfg.Logging.Type == "" {
		cfg.Logging.Type = DefaultConfig.Logging.Type
	}

	return cfg
}
