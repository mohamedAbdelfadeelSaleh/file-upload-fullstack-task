package handler_test

import (
	"backend/internal/handler"
	"backend/internal/service"
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	//"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	//"gorm.io/driver/sqlite"
	//"gorm.io/gorm"
)

type MockUploadService struct {
	mock.Mock
}

func (m *MockUploadService) ProcessCSV(filePath string) error {
	args := m.Called(filePath)
	return args.Error(0)
}

func (m *MockUploadService) GetFileProgress(fileName string) *service.ProgressInfo {
	args := m.Called(fileName)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*service.ProgressInfo)
}

func (m *MockUploadService) GetAllFileProgress() []*service.ProgressInfo {
	args := m.Called()
	return args.Get(0).([]*service.ProgressInfo)
}

func (m *MockUploadService) RegisterProgressListener(ch chan *service.ProgressInfo) {
	m.Called(ch)
}

func (m *MockUploadService) UnregisterProgressListener(ch chan *service.ProgressInfo) {
	m.Called(ch)
}

func TestUploadCSV(t *testing.T) {
	// Setup mock service
	mockService := new(MockUploadService)
	mockService.On("ProcessCSV", mock.AnythingOfType("string")).Return(nil)

	handler := handler.NewUploadHandler(mockService)

	// Create a test file
	csvContent := "StudentID,StudentName,Subject,Grade\nS001,Alice,Math,95"

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "test.csv")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(csvContent))
	writer.Close()

	// Create the HTTP request
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Record the response
	w := httptest.NewRecorder()

	// Call the handler
	handler.UploadCSV(w, req)

	// Check the response
	resp := w.Result()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	// Check that the mock was called
	mockService.AssertCalled(t, "ProcessCSV", mock.AnythingOfType("string"))

	// Check that the uploads directory was created
	_, err = os.Stat("uploads")
	assert.NoError(t, err)

	// Clean up
	os.RemoveAll("uploads")
}

func TestUploadCSV_NoFiles(t *testing.T) {
	mockService := new(MockUploadService)
	handler := handler.NewUploadHandler(mockService)

	// Create multipart request with no files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	handler.UploadCSV(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), "No files uploaded")
}

func TestUploadCSV_FileTooLarge(t *testing.T) {
	mockService := new(MockUploadService)
	handler := handler.NewUploadHandler(mockService)

	// Create a large file that exceeds the limit
	largeData := make([]byte, 101*1024*1024) // 101 MB

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "large.csv")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(largeData)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	handler.UploadCSV(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}
