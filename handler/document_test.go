package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	data map[string]string
}

func (m *mockStorage) Get(key string, _ bool) (string, error) {
	val, exists := m.data[key]
	if !exists {
		return "", fmt.Errorf("not found")
	}
	return val, nil
}

func (m *mockStorage) Set(key, value string, _ bool) error {
	m.data[key] = value
	return nil
}

func (m *mockStorage) Close() error {
	return nil
}

type mockKeyGenerator struct {
	fixedKey string
}

func (m *mockKeyGenerator) Generate(_ int) string {
	return m.fixedKey
}

func setupHandler() *DocumentHandler {
	store := &mockStorage{data: make(map[string]string)}
	keyGen := &mockKeyGenerator{fixedKey: "test123"}
	return NewDocumentHandler(6, 1024, store, keyGen)
}

func sendRequest(handler http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestHandlePost(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := bytes.NewBufferString("test content")
	resp := sendRequest(router, http.MethodPost, "/documents", body)

	require.Equal(t, http.StatusOK, resp.Code)

	var responseData map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&responseData))
	require.Equal(t, "test123", responseData["key"])
}

func TestHandleGet_NotFound(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	resp := sendRequest(router, http.MethodGet, "/documents/unknown", nil)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandleGet_Found(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Add test document
	handler.Store.Set("test123", "stored content", false)

	resp := sendRequest(router, http.MethodGet, "/documents/test123", nil)

	require.Equal(t, http.StatusOK, resp.Code)

	var responseData map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&responseData))
	require.Equal(t, "stored content", responseData["data"])

	// Test HEAD request
	resp = sendRequest(router, http.MethodHead, "/documents/test123", nil)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Empty(t, resp.Body.String())
}

func TestHandleRawGet_Found(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Add test document
	handler.Store.Set("test123", "raw data", false)

	resp := sendRequest(router, http.MethodGet, "/raw/test123", nil)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "raw data", resp.Body.String())

	// Test HEAD request
	resp = sendRequest(router, http.MethodHead, "/raw/test123", nil)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Empty(t, resp.Body.String())
}

func TestHandleRawGet_NotFound(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	resp := sendRequest(router, http.MethodGet, "/raw/unknown", nil)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandlePutLog(t *testing.T) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := bytes.NewBufferString("log entry")
	resp := sendRequest(router, http.MethodPut, "/log", body)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "https://")
	require.Contains(t, resp.Body.String(), "test123")
}

func TestHandlePutLog_ExceedsMaxLength(t *testing.T) {
	handler := NewDocumentHandler(6, 10, &mockStorage{data: make(map[string]string)}, &mockKeyGenerator{fixedKey: "test123"})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := bytes.NewBufferString("this content is too long")
	resp := sendRequest(router, http.MethodPut, "/log", body)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "{\"message\": \"Document exceeds maximum length.\"}\n", resp.Body.String())
}

func TestHandlePost_ExceedsMaxLength(t *testing.T) {
	handler := NewDocumentHandler(6, 10, &mockStorage{data: make(map[string]string)}, &mockKeyGenerator{fixedKey: "test123"})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := bytes.NewBufferString("this content is too long")
	resp := sendRequest(router, http.MethodPost, "/documents", body)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "{\"message\": \"Document exceeds maximum length.\"}\n", resp.Body.String())
}

func BenchmarkHandlePost(b *testing.B) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Disable logging for benchmark
	log.Logger = log.Level(zerolog.Disabled)

	body := bytes.NewBufferString("benchmark content")
	for i := 0; i < b.N; i++ {
		sendRequest(router, http.MethodPost, "/documents", body)
	}
}

func BenchmarkHandleGet(b *testing.B) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Disable logging for benchmark
	log.Logger = log.Level(zerolog.Disabled)

	// Add document
	handler.Store.Set("test123", "benchmark data", false)

	for i := 0; i < b.N; i++ {
		sendRequest(router, http.MethodGet, "/documents/test123", nil)
	}
}

func BenchmarkHandlePutLog(b *testing.B) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Disable logging for benchmark
	log.Logger = log.Level(zerolog.Disabled)

	body := bytes.NewBufferString("benchmark log entry")
	for i := 0; i < b.N; i++ {
		sendRequest(router, http.MethodPut, "/log", body)
	}
}

func BenchmarkHandleRawGet(b *testing.B) {
	handler := setupHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Disable logging for benchmark
	log.Logger = log.Level(zerolog.Disabled)

	// Add document
	handler.Store.Set("test123", "benchmark data", false)

	for i := 0; i < b.N; i++ {
		sendRequest(router, http.MethodGet, "/raw/test123", nil)
	}
}
