package main

import (
	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/handler"
	"backend/internal/service"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize database
	db := database.InitDB()

	// Initialize services
	studentService := service.NewStudentService(db)
	uploadService := service.NewUploadService(db)

	// Initialize handlers
	studentHandler := handler.NewStudentHandler(studentService)
	uploadHandler := handler.NewUploadHandler(uploadService)

	// Setup router
	r := mux.NewRouter()

	r.HandleFunc("/upload", uploadHandler.UploadCSV).Methods("POST")

	r.HandleFunc("/students", studentHandler.ListStudents).Methods("GET")

	////////////////////////////////////////////////////////////////////////////////////////
	progressHandler := handler.NewProgressHandler(uploadService)

	r.HandleFunc("/progress/sse", progressHandler.SSEProgress).Methods("GET")
	//////////////////////////////////////////////////////////////////////////////////////
	// Create uploads directory
	if err := os.Mkdir("uploads", os.ModePerm); err != nil && !os.IsExist(err) {
		log.Fatal("Failed to create uploads directory:", err)
	}

	// Start server
	log.Println("Server running on port 8080")
	err := http.ListenAndServe(":8080", handlers.CORS(handlers.AllowedOrigins([]string{"http://localhost:3000"}))(r))
	if err != nil {
		return
	}
}
