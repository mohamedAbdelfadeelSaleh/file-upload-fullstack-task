package handler_test

import (
	"backend/internal/handler"
	"backend/internal/model"
	"backend/internal/service"
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListStudents(t *testing.T) {
	// Setup
	db := setupTestDB()
	studentService := service.NewStudentService(db)
	studentHandler := handler.NewStudentHandler(studentService)

	// Insert test data
	students := []model.Student{
		{StudentName: "John Doe", Subject: "Math", Grade: 90},
		{StudentName: "Jane Doe", Subject: "Science", Grade: 85},
		{StudentName: "Alice", Subject: "Math", Grade: 95},
	}
	for _, student := range students {
		db.Create(&student)
	}

	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedStatus int
		expectedLen    int
	}{
		{"All students", map[string]string{}, http.StatusOK, 3},
		{"Filter by name", map[string]string{"student_name": "John"}, http.StatusOK, 1},
		{"Filter by subject", map[string]string{"subject": "Math"}, http.StatusOK, 2},
		{"Filter by grade range", map[string]string{"grade_min": "85", "grade_max": "90"}, http.StatusOK, 2},
		{"Pagination", map[string]string{"page": "1", "limit": "2"}, http.StatusOK, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/students", nil)
			if err != nil {
				t.Fatal(err)
			}

			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(studentHandler.ListStudents)
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("ListStudents() status = %v, want %v", rr.Code, tt.expectedStatus)
			}

			var response map[string]interface{}
			err = json.NewDecoder(rr.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			data := response["data"].([]interface{})
			if len(data) != tt.expectedLen {
				t.Errorf("ListStudents() got = %v, want %v", len(data), tt.expectedLen)
			}
		})
	}
}

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}
	db.AutoMigrate(&model.Student{})
	return db
}
