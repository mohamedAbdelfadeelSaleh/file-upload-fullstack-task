package handler

import (
	"backend/internal/service"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	//"time"
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

// SSEProgress streams progress updates to the client using Server-Sent Events (SSE)
func (h *ProgressHandler) SSEProgress(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to send progress updates
	progressChan := make(chan *service.ProgressInfo)
	defer close(progressChan)

	// Register the client to receive progress updates
	h.uploadService.RegisterProgressListener(progressChan)
	defer h.uploadService.UnregisterProgressListener(progressChan)

	// Send progress updates to the client
	for {
		select {
		case progress := <-progressChan:
			// Send progress update as an SSE event
			data, err := json.Marshal(progress)
			if err != nil {
				log.Println("Error marshaling progress:", err)
				continue
			}
			_, err = w.Write([]byte("data: " + string(data) + "\n\n"))
			if err != nil {
				log.Println("Error writing SSE data:", err)
				return
			}
			w.(http.Flusher).Flush() // Flush the response to send the data immediately

		case <-r.Context().Done():
			// Client disconnected
			log.Println("Client disconnected")
			return
		}
	}
}
