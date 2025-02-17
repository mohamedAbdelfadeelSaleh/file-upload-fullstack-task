package handler_test

import (
	"backend/internal/handler"
	"backend/internal/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	//"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProgressService struct {
	mock.Mock
}

func (m *MockProgressService) GetFileProgress(fileName string) *service.ProgressInfo {
	args := m.Called(fileName)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*service.ProgressInfo)
}

func (m *MockProgressService) GetAllFileProgress() []*service.ProgressInfo {
	args := m.Called()
	return args.Get(0).([]*service.ProgressInfo)
}

func (m *MockProgressService) RegisterProgressListener(ch chan *service.ProgressInfo) {
	m.Called(ch)
}

func (m *MockProgressService) UnregisterProgressListener(ch chan *service.ProgressInfo) {
	m.Called(ch)
}

func TestGetFileProgress(t *testing.T) {
	mockService := new(MockProgressService)

	// Test with existing file
	progress := &service.ProgressInfo{
		FileName:     "test.csv",
		TotalRecords: 100,
		Processed:    50,
		Status:       "processing",
	}

	mockService.On("GetFileProgress", "test.csv").Return(progress)

	handler := handler.NewProgressHandler(mockService)

	// Create request
	req := httptest.NewRequest("GET", "/progress?fileName=test.csv", nil)
	w := httptest.NewRecorder()

	// Create router to parse query parameters
	router := mux.NewRouter()
	router.HandleFunc("/progress", handler.GetFileProgress)
	router.ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response body
	var response service.ProgressInfo
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Equal(t, "test.csv", response.FileName)
	assert.Equal(t, 100, response.TotalRecords)
	assert.Equal(t, 50, response.Processed)
	assert.Equal(t, "processing", response.Status)

	// Test with non-existent file
	mockService.On("GetFileProgress", "nonexistent.csv").Return(nil)

	req = httptest.NewRequest("GET", "/progress?fileName=nonexistent.csv", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test without filename parameter
	req = httptest.NewRequest("GET", "/progress", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	resp = w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetAllProgress(t *testing.T) {
	mockService := new(MockProgressService)

	// Set up test data
	progress1 := &service.ProgressInfo{
		FileName:     "file1.csv",
		TotalRecords: 100,
		Processed:    75,
		Status:       "processing",
	}

	progress2 := &service.ProgressInfo{
		FileName:     "file2.csv",
		TotalRecords: 200,
		Processed:    200,
		Status:       "completed",
	}

	progressList := []*service.ProgressInfo{progress1, progress2}

	mockService.On("GetAllFileProgress").Return(progressList)

	handler := NewProgressHandler(mockService)

	// Create request
	req := httptest.NewRequest("GET", "/progress/all", nil)
	w := httptest.NewRecorder()

	// Call handler
	handler.GetAllProgress(w, req)

	// Check response
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response body
	var response []*service.ProgressInfo
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Len(t, response, 2)
	assert.Equal(t, "file1.csv", response[0].FileName)
	assert.Equal(t, "file2.csv", response[1].FileName)
	assert.Equal(t, "processing", response[0].Status)
	assert.Equal(t, "completed", response[1].Status)

	// Verify mock was called
	mockService.AssertExpectations(t)
}

func TestSSEProgress(t *testing.T) {
	mockService := new(MockProgressService)

	// Expect register and unregister calls
	mockService.On("RegisterProgressListener", mock.AnythingOfType("chan *service.ProgressInfo")).Return()
	mockService.On("UnregisterProgressListener", mock.AnythingOfType("chan *service.ProgressInfo")).Return()

	handler := NewProgressHandler(mockService)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create request with the cancellable context
	req := httptest.NewRequest("GET", "/progress/sse", nil).WithContext(ctx)

	// Create recorder
	w := httptest.NewRecorder()

	// Start the handler in a goroutine because it will block
	go func() {
		handler.SSEProgress(w, req)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to simulate client disconnection
	cancel()

	// Wait for handler to finish
	time.Sleep(50 * time.Millisecond)

	// Check headers
	resp := w.Result()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))

	// Verify that register and unregister were called
	mockService.AssertExpectations(t)
}

// Additional test to check actual data sending
func TestSSEProgressDataSending(t *testing.T) {
	mockService := new(MockProgressService)

	// Create a channel we can control
	progressChan := make(chan *service.ProgressInfo, 1)

	// Setup mock to use our channel
	mockService.On("RegisterProgressListener", mock.AnythingOfType("chan *service.ProgressInfo")).
		Run(func(args mock.Arguments) {
			// Get the channel passed to RegisterProgressListener
			ch := args.Get(0).(chan *service.ProgressInfo)
			// Copy our progress to that channel
			go func() {
				for p := range progressChan {
					ch <- p
				}
			}()
		}).
		Return()

	mockService.On("UnregisterProgressListener", mock.AnythingOfType("chan *service.ProgressInfo")).Return()

	handler := NewProgressHandler(mockService)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Create request
	req := httptest.NewRequest("GET", "/progress/sse", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start handler in goroutine
	go handler.SSEProgress(w, req)

	// Send a progress update
	progress := &service.ProgressInfo{
		FileName:     "test.csv",
		TotalRecords: 100,
		Processed:    50,
		Status:       "processing",
	}

	progressChan <- progress

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check response body
	responseBody := w.Body.String()

	// Should contain the progress data as an SSE event
	expectedData, _ := json.Marshal(progress)
	expectedEvent := "data: " + string(expectedData) + "\n\n"

	assert.Contains(t, responseBody, expectedEvent)

	// Close our test channel and cancel context
	close(progressChan)
	cancel()

	// Verify mock expectations
	mockService.AssertExpectations(t)
}
