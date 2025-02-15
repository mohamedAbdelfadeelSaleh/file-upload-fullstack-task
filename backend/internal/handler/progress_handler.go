package handler

import (
	"backend/internal/service"
	"encoding/json"
	"net/http"
	"path/filepath"
)

type ProgressHandler struct {
	uploadService *service.UploadService
}

func NewProgressHandler(uploadService *service.UploadService) *ProgressHandler {
	return &ProgressHandler{uploadService: uploadService}
}

// GetFileProgress returns the progress for a specific file
func (h *ProgressHandler) GetFileProgress(w http.ResponseWriter, r *http.Request) {
	fileName := r.URL.Query().Get("fileName")
	if fileName == "" {
		http.Error(w, "fileName parameter is required", http.StatusBadRequest)
		return
	}

	progress := h.uploadService.GetFileProgress(filepath.Base(fileName))
	if progress == nil {
		http.Error(w, "File not found or not being processed", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

// GetAllProgress returns the progress for all files being processed
func (h *ProgressHandler) GetAllProgress(w http.ResponseWriter, r *http.Request) {
	progress := h.uploadService.GetAllFileProgress()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}
