package handler

import (
	"backend/internal/service"
	"encoding/json"
	_ "math"
	"net/http"
	"strconv"
)

type StudentHandler struct {
	studentService *service.StudentService
}

func NewStudentHandler(studentService *service.StudentService) *StudentHandler {
	return &StudentHandler{studentService: studentService}
}

func (h *StudentHandler) ListStudents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit < 1 {
		limit = 10
	}
	sortBy := query.Get("sort_by")
	if sortBy == "" {
		sortBy = "student_name"
	}
	sortOrder := query.Get("sort_order")
	if sortOrder == "" {
		sortOrder = "asc"
	}
	studentName := query.Get("student_name")
	subject := query.Get("subject")
	gradeMin, _ := strconv.Atoi(query.Get("grade_min"))
	gradeMax, _ := strconv.Atoi(query.Get("grade_max"))

	students, totalCount, totalPages, err := h.studentService.ListStudents(page, limit, sortBy, sortOrder, studentName, subject, gradeMin, gradeMax)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data":       students,
		"page":       page,
		"limit":      limit,
		"total":      totalCount,
		"totalPages": totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}
