package service

import (
	"backend/internal/model"
	"backend/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"math"
	"testing"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}
	db.AutoMigrate(&model.Student{})
	return db
}

func TestListStudents(t *testing.T) {
	db := setupTestDB()
	studentService := service.NewStudentService(db)

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
		name        string
		page        int
		limit       int
		sortBy      string
		sortOrder   string
		studentName string
		subject     string
		gradeMin    int
		gradeMax    int
		expectedLen int
	}{
		{"All students", 1, 10, "student_name", "asc", "", "", 0, 0, 3},
		{"Filter by name", 1, 10, "student_name", "asc", "John", "", 0, 0, 1},
		{"Filter by subject", 1, 10, "student_name", "asc", "", "Math", 0, 0, 2},
		{"Filter by grade range", 1, 10, "student_name", "asc", "", "", 85, 90, 2},
		{"Pagination", 1, 2, "student_name", "asc", "", "", 0, 0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			students, totalCount, totalPages, err := studentService.ListStudents(tt.page, tt.limit, tt.sortBy, tt.sortOrder, tt.studentName, tt.subject, tt.gradeMin, tt.gradeMax)
			if err != nil {
				t.Fatalf("ListStudents() error = %v", err)
			}
			if len(students) != tt.expectedLen {
				t.Errorf("ListStudents() got = %v, want %v", len(students), tt.expectedLen)
			}
			if totalCount != int64(len(students)) {
				t.Errorf("ListStudents() totalCount = %v, want %v", totalCount, len(students))
			}
			if totalPages != int(math.Ceil(float64(totalCount)/float64(tt.limit))) {
				t.Errorf("ListStudents() totalPages = %v, want %v", totalPages, int(math.Ceil(float64(totalCount)/float64(tt.limit))))
			}
		})
	}
}
