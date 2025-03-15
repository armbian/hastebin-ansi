package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig_DefaultValues(t *testing.T) {
	cfg := NewConfig("nonexistent.yaml") // Should use defaults since file doesn't exist

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 7777, cfg.Port)
	assert.Equal(t, 10, cfg.KeyLength)
	assert.Equal(t, "phonetic", cfg.KeyGenerator)
	assert.Equal(t, "file", cfg.Storage.Type)
	assert.Equal(t, "data", cfg.Storage.FilePath)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Type)
}

func TestNewConfig_OverrideWithEnvVars(t *testing.T) {
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("PORT", "8080")
	os.Setenv("KEY_LENGTH", "15")
	os.Setenv("MAX_LENGTH", "5000000")
	os.Setenv("STORAGE_TYPE", "redis")
	os.Setenv("STORAGE_HOST", "localhost")
	os.Setenv("STORAGE_PORT", "6379")
	os.Setenv("LOGGING_LEVEL", "debug")

	defer os.Clearenv()

	cfg := NewConfig("nonexistent.yaml") // Load with environment variables

	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, 15, cfg.KeyLength)
	assert.Equal(t, 5000000, cfg.MaxLength)
	assert.Equal(t, "redis", cfg.Storage.Type)
	assert.Equal(t, "localhost", cfg.Storage.Host)
	assert.Equal(t, 6379, cfg.Storage.Port)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestNewConfig_LoadFromYAML(t *testing.T) {
	yamlContent := `
host: "192.168.1.1"
port: 9090
key_length: 20
storage:
  type: "mongodb"
  host: "mongo.example.com"
  port: 27017
logging:
  level: "warn"
  type: "json"
`

	// Write to a temporary file
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(yamlContent))
	assert.NoError(t, err)
	tmpFile.Close()

	cfg := NewConfig(tmpFile.Name()) // Load config from YAML file

	assert.Equal(t, "192.168.1.1", cfg.Host)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 20, cfg.KeyLength)
	assert.Equal(t, "mongodb", cfg.Storage.Type)
	assert.Equal(t, "mongo.example.com", cfg.Storage.Host)
	assert.Equal(t, 27017, cfg.Storage.Port)
	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Type)
}
