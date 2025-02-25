package handler

import (
	"backend/internal/service"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type UploadHandler struct {
	uploadService *service.UploadService
}

func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

func (h *UploadHandler) UploadCSV(w http.ResponseWriter, r *http.Request) {
	// Ensure uploads directory exists
	if err := os.MkdirAll("uploads", 0755); err != nil {
		http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		http.Error(w, "File too large or bad request", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // Protects fileNames from concurrent writes
	fileNames := make([]string, 0, len(files))

	for _, handler := range files {
		wg.Add(1)
		go func(handler *multipart.FileHeader) {
			defer wg.Done()

			file, err := handler.Open()
			if err != nil {
				log.Println("Error opening file:", err)
				return
			}
			defer file.Close()

			savePath := filepath.Join("uploads", handler.Filename)
			outFile, err := os.Create(savePath)
			if err != nil {
				log.Println("Error saving the file:", err)
				return
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, file)
			if err != nil {
				log.Println("Error writing file:", err)
				return
			}

			// Synchronize access to fileNames
			mu.Lock()
			fileNames = append(fileNames, handler.Filename)
			mu.Unlock()

			// Process the CSV file
			if err := h.uploadService.ProcessCSV(savePath); err != nil {
				log.Printf("Error processing file %s: %v", savePath, err)
			}
		}(handler)
	}

	go func() {
		wg.Wait()
		log.Println("All files processed")
	}()

	// Return a response with the file names that were processed
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]interface{}{
		"message": "Files uploaded successfully and processing started",
		//"files":   fileNames,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Println("Error encoding response:", err)
	}
}
