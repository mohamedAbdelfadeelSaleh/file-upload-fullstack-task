package service

import (
	"backend/internal/model"
	"gorm.io/gorm"
	"math"
)

type StudentService struct {
	db *gorm.DB
}

func NewStudentService(db *gorm.DB) *StudentService {
	return &StudentService{db: db}
}

func (s *StudentService) ListStudents(page, limit int, sortBy, sortOrder, studentName, subject string, gradeMin, gradeMax int) ([]model.Student, int64, int, error) {
	var students []model.Student
	dbQuery := s.db.Model(&model.Student{})

	// Apply filters
	if studentName != "" {
		dbQuery = dbQuery.Where("student_name ILIKE ?", "%"+studentName+"%")
	}
	if subject != "" {
		dbQuery = dbQuery.Where("subject = ?", subject)
	}
	if gradeMin > 0 {
		dbQuery = dbQuery.Where("grade >= ?", gradeMin)
	}
	if gradeMax > 0 {
		dbQuery = dbQuery.Where("grade <= ?", gradeMax)
	}

	// Apply sorting
	orderClause := sortBy + " " + sortOrder
	dbQuery = dbQuery.Order(orderClause)

	// Pagination
	var totalCount int64
	dbQuery.Count(&totalCount)
	dbQuery.Offset((page - 1) * limit).Limit(limit).Find(&students)

	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))

	return students, totalCount, totalPages, nil
}
