package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/armbian/ansi-hastebin/internal/keygenerator"
	"github.com/armbian/ansi-hastebin/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// DocumentHandler manages document operations
type DocumentHandler struct {
	KeyLength    int
	MaxLength    int
	Store        storage.Storage
	KeyGenerator keygenerator.KeyGenerator
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
	key := strings.Split(chi.URLParam(r, "id"), ".")[0]
	data, err := h.Store.Get(key, false)

	if data != "" && err == nil {
		logrus.WithField("key", key).Info("Retrieved document")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"data": data, "key": key})
	} else {
		logrus.WithField("key", key).Warn("Document not found")
		http.Error(w, `{"message": "Document not found."}`, http.StatusNotFound)
	}
}

// Handle retrieving raw document
func (h *DocumentHandler) HandleRawGet(w http.ResponseWriter, r *http.Request) {

	key := strings.Split(chi.URLParam(r, "id"), ".")[0]
	data, err := h.Store.Get(key, false)
	if data != "" && err == nil {
		logrus.WithField("key", key).Info("Retrieved raw document")
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte(data))
	} else {
		logrus.WithField("key", key).Warn("Raw document not found")
		http.Error(w, `{"message": "Document not found."}`, http.StatusNotFound)
	}
}

// Handle adding a new document (POST)
func (h *DocumentHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	var buffer strings.Builder
	if err := h.readBody(r, &buffer); err != nil {
		http.Error(w, `{"message": "Error reading request body."}`, http.StatusInternalServerError)
		return
	}

	if h.MaxLength > 0 && buffer.Len() > h.MaxLength {
		logrus.Warn("Document exceeds max length")
		http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusBadRequest)
		return
	}

	key := h.KeyGenerator.Generate(h.KeyLength)
	h.Store.Set(key, buffer.String(), false)

	logrus.WithField("key", key).Info("Added document")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"key": key})
}

// Handle PUT request that returns a direct link
func (h *DocumentHandler) HandlePutLog(w http.ResponseWriter, r *http.Request) {
	var buffer strings.Builder
	if err := h.readBody(r, &buffer); err != nil {
		http.Error(w, `{"message": "Error reading request body."}`, http.StatusInternalServerError)
		return
	}

	if h.MaxLength > 0 && buffer.Len() > h.MaxLength {
		logrus.Warn("Document exceeds max length")
		http.Error(w, `{"message": "Document exceeds maximum length."}`, http.StatusBadRequest)
		return
	}

	key := h.KeyGenerator.Generate(h.KeyLength)
	h.Store.Set(key, buffer.String(), false)

	logrus.WithField("key", key).Info("Added document with log link")
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "\nhttps://%s/%s\n\n", r.Host, key)
}

// Reads body from the request
func (h *DocumentHandler) readBody(r *http.Request, buffer *strings.Builder) error {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.ParseMultipartForm(32 << 20)
		if val := r.FormValue("data"); val != "" {
			buffer.WriteString(val)
		}
	} else {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			logrus.Error("Connection error: ", err)
			return err
		}
		buffer.WriteString(string(data))
	}
	return nil
}
