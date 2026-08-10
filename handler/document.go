package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/armbian/ansi-hastebin/internal/keygenerator"
	"github.com/armbian/ansi-hastebin/internal/storage"
	"github.com/armbian/ansi-hastebin/internal/unsafeconv"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
)

var (
	requestBodyBufferPool = sync.Pool{
		New: func() any { return make([]byte, 32<<10) },
	}

	pasteCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hastebin_paste_created",
		Help: "The total number of pastes created",
	})

	pasteRead = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hastebin_paste_read",
		Help: "The total number of pastes read",
	})
)

// DocumentHandler manages document operations
type DocumentHandler struct {
	KeyLength          int
	MaxLength          int
	Store              storage.Storage
	KeyGenerator       keygenerator.KeyGenerator
	DeleteAfterEnabled bool
}

func NewDocumentHandler(keyLength, maxLength int, store storage.Storage, keyGenerator keygenerator.KeyGenerator) *DocumentHandler {
	return &DocumentHandler{
		KeyLength:    keyLength,
		MaxLength:    maxLength,
		Store:        store,
		KeyGenerator: keyGenerator,
	}
}

// RegisterRoutes registers document routes
func (h *DocumentHandler) RegisterRoutes(r chi.Router) {
	r.Get("/raw/{id}", h.HandleRawGet)
	r.Head("/raw/{id}", h.HandleRawGet)

	r.Post("/log", h.HandlePutLog)
	r.Put("/log", h.HandlePutLog)

	r.Post("/documents", h.HandlePost)

	r.Get("/documents/{id}", h.HandleGet)
	r.Head("/documents/{id}", h.HandleGet)
}

// Handle retrieving a document
func (h *DocumentHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key := id
	if idx := strings.IndexByte(id, '.'); idx >= 0 {
		key = id[:idx]
	}

	data, err := h.Store.Get(key, false)

	if data != "" && err == nil {
		log.Info().Str("key", key).Msg("Retrieved document")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		pasteRead.Inc()
		json.NewEncoder(w).Encode(struct {
			Data string `json:"data"`
			Key  string `json:"key"`
		}{
			Data: data,
			Key:  key,
		})
	} else {
		log.Info().Str("key", key).Msg("Document not found")
		http.Error(w, `{"message": "Document not found."}`, http.StatusNotFound)
	}
}

// Handle retrieving raw document
func (h *DocumentHandler) HandleRawGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key := id
	if idx := strings.IndexByte(id, '.'); idx >= 0 {
		key = id[:idx]
	}

	data, err := h.Store.Get(key, false)

	if data != "" && err == nil {
		log.Info().Str("key", key).Msg("Retrieved raw document")
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		pasteRead.Inc()
		w.Write(unsafeconv.UnsafeBytes(data))
	} else {
		log.Info().Str("key", key).Msg("Raw document not found")
		http.Error(w, `{"message": "Document not found."}`, http.StatusNotFound)
	}
}

// Handle adding a new document (POST)
func (h *DocumentHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	var buffer strings.Builder
	if err := h.readBody(w, r, &buffer); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"message": "Error reading request body."}`, http.StatusInternalServerError)
		return
	}

	if h.MaxLength > 0 && buffer.Len() > h.MaxLength {
		log.Info().Str("key", "").Msg("Document exceeds max length")
		http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusBadRequest)
		return
	}

	key := h.KeyGenerator.Generate(h.KeyLength)
	expiresAt, err := h.storeDocument(key, buffer.String(), r.Header.Get("X-Delete-After"))
	if err != nil {
		if errors.Is(err, errInvalidDeleteAfter) {
			http.Error(w, `{"message": "Invalid delete-after value."}`, http.StatusBadRequest)
			return
		}
		if errors.Is(err, errDeleteAfterDisabled) {
			http.Error(w, `{"message": "Delete-after is disabled."}`, http.StatusForbidden)
			return
		}
		log.Error().Err(err).Str("key", key).Msg("Failed to store document")
		http.Error(w, `{"message": "Error storing document."}`, http.StatusInternalServerError)
		return
	}

	log.Info().Str("key", key).Msg("Added document")

	pasteCreated.Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Key       string     `json:"key"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}{
		Key: key, ExpiresAt: expiresAt,
	})
}

// Handle PUT request that returns a direct link
func (h *DocumentHandler) HandlePutLog(w http.ResponseWriter, r *http.Request) {
	var buffer strings.Builder
	if err := h.readBody(w, r, &buffer); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"message": "Error reading request body."}`, http.StatusInternalServerError)
		return
	}

	if h.MaxLength > 0 && buffer.Len() > h.MaxLength {
		log.Info().Str("key", "").Msg("Document exceeds max length")
		http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusBadRequest)
		return
	}

	key := h.KeyGenerator.Generate(h.KeyLength)
	_, err := h.storeDocument(key, buffer.String(), r.Header.Get("X-Delete-After"))
	if err != nil {
		if errors.Is(err, errInvalidDeleteAfter) {
			http.Error(w, `{"message": "Invalid delete-after value."}`, http.StatusBadRequest)
			return
		}
		if errors.Is(err, errDeleteAfterDisabled) {
			http.Error(w, `{"message": "Delete-after is disabled."}`, http.StatusForbidden)
			return
		}
		log.Error().Err(err).Str("key", key).Msg("Failed to store document")
		http.Error(w, `{"message": "Error storing document."}`, http.StatusInternalServerError)
		return
	}

	log.Info().Str("key", key).Msg("Added document with log link")
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "\nhttps://%s/%s\n\n", r.Host, key)
}

var errInvalidDeleteAfter = errors.New("invalid delete-after")
var errDeleteAfterDisabled = errors.New("delete-after is disabled")

func (h *DocumentHandler) storeDocument(key, value, deleteAfter string) (*time.Time, error) {
	if deleteAfter == "" || deleteAfter == "never" {
		return nil, h.Store.Set(key, value, false)
	}
	if !h.DeleteAfterEnabled {
		return nil, errDeleteAfterDisabled
	}
	ttl, err := time.ParseDuration(deleteAfter)
	if err != nil || ttl <= 0 || ttl > 30*24*time.Hour {
		return nil, errInvalidDeleteAfter
	}
	store, ok := h.Store.(storage.DeleteAfterStorage)
	if !ok {
		return nil, errors.New("storage backend does not support delete-after")
	}
	if err := store.SetWithDeleteAfter(key, value, ttl); err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(ttl).UTC()
	return &expiresAt, nil
}

// Reads body from the request
func (h *DocumentHandler) readBody(w http.ResponseWriter, r *http.Request, buffer *strings.Builder) error {
	limit := int64(h.MaxLength)
	if limit <= 0 {
		limit = 10 << 20 // Default 10MB limit if not specified to avoid reading infinite body.
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			return err
		}
		if val, ok := r.Form["data"]; ok && len(val) > 0 {
			buffer.WriteString(val[0])
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else {
		copyBuffer := requestBodyBufferPool.Get().([]byte)
		defer requestBodyBufferPool.Put(copyBuffer)
		_, err := io.CopyBuffer(buffer, r.Body, copyBuffer)
		if err != nil {
			log.Error().Err(err).Msg("Error reading request body")
			return err
		}
	}
	return nil
}
