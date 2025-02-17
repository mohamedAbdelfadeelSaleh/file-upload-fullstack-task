package handler

import (
	"backend/internal/service"
	"encoding/json"
	"log"
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

	// Calculate percentage
	var percentage float64
	if progress.TotalRecords > 0 {
		percentage = float64(progress.Processed) / float64(progress.TotalRecords) * 100
	}

	// Add percentage to the response
	response := struct {
		*service.ProgressInfo
		Percentage float64 `json:"percentage"`
	}{
		ProgressInfo: progress,
		Percentage:   percentage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAllProgress returns the progress for all files being processed
func (h *ProgressHandler) GetAllProgress(w http.ResponseWriter, r *http.Request) {
	progressList := h.uploadService.GetAllFileProgress()

	// Calculate percentage for each file
	type ProgressWithPercentage struct {
		*service.ProgressInfo
		Percentage float64 `json:"percentage"`
	}

	response := make([]ProgressWithPercentage, 0, len(progressList))
	for _, progress := range progressList {
		var percentage float64
		if progress.TotalRecords > 0 {
			percentage = float64(progress.Processed) / float64(progress.TotalRecords) * 100
		}

		response = append(response, ProgressWithPercentage{
			ProgressInfo: progress,
			Percentage:   percentage,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SSEProgress streams progress updates to the client using Server-Sent Events (SSE)
func (h *ProgressHandler) SSEProgress(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to receive progress updates
	progressChan := make(chan *service.ProgressInfo)
	defer close(progressChan)

	// Register the client to receive progress updates
	h.uploadService.RegisterProgressListener(progressChan)
	defer h.uploadService.UnregisterProgressListener(progressChan)

	// Send progress updates to the client
	for {
		select {
		case progress := <-progressChan:
			// Calculate percentage
			var percentage float64
			if progress.TotalRecords > 0 {
				percentage = float64(progress.Processed) / float64(progress.TotalRecords) * 100
			}

			// Create a response that includes the percentage
			response := struct {
				*service.ProgressInfo
				Percentage float64 `json:"percentage"`
			}{
				ProgressInfo: progress,
				Percentage:   percentage,
			}

			// Marshal the response
			data, err := json.Marshal(response)
			if err != nil {
				log.Println("Error marshaling progress:", err)
				continue
			}

			// Send the SSE event
			_, err = w.Write([]byte("data: " + string(data) + "\n\n"))
			if err != nil {
				log.Println("Error writing SSE data:", err)
				return
			}
			w.(http.Flusher).Flush() // Flush the response to send the data immediately

		case <-r.Context().Done():
			return
		}
	}
}
