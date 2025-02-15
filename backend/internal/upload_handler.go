package handler

import (
	"backend/internal/service"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	//"time"
)

type UploadHandler struct {
	uploadService *service.UploadService
}

func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

func (h *UploadHandler) UploadCSV(w http.ResponseWriter, r *http.Request) {
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

	for _, handler := range files {
		file, err := handler.Open()
		if err != nil {
			log.Println("Error opening file:", err)
			continue
		}

		savePath := filepath.Join("uploads", handler.Filename)
		outFile, err := os.Create(savePath)
		if err != nil {
			log.Println("Error saving the file:", err)
			file.Close()
			continue
		}

		//startTime := time.Now()
		_, err = io.Copy(outFile, file)
		if err != nil {
			log.Println("Error writing file:", err)
			file.Close()
			outFile.Close()
			continue
		}
		//uploadTime := time.Since(startTime)
		file.Close()
		outFile.Close()
		//log.Printf("upload time:", err)

		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()
			h.uploadService.ProcessCSV(filePath)
		}(savePath)
	}

	go func() {
		wg.Wait()
		log.Println("All files processed")
	}()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "Files uploaded successfully and processing started")
}
