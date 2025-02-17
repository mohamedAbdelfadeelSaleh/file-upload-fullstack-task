package service_test

import (
	//"backend/internal/handler"
	"backend/internal/model"
	"backend/internal/service"
	//"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&model.Student{})
	return db
}

func TestNewUploadService(t *testing.T) {
	db := setupTestDB()
	service := service.NewUploadService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.NotNil(t, service.fileProgressMap)
	assert.NotNil(t, service.progressListeners)
}

func TestRegisterAndUnregisterProgressListener(t *testing.T) {
	db := setupTestDB()
	service := service.NewUploadService(db)

	ch := make(chan *ProgressInfo)

	// Register
	service.RegisterProgressListener(ch)

	service.listenerLock.RLock()
	assert.True(t, service.progressListeners[ch])
	service.listenerLock.RUnlock()

	// Unregister
	service.UnregisterProgressListener(ch)

	service.listenerLock.RLock()
	assert.False(t, service.progressListeners[ch])
	service.listenerLock.RUnlock()
}

func TestBroadcastProgress(t *testing.T) {
	db := setupTestDB()
	service := service.NewUploadService(db)

	ch := make(chan *ProgressInfo, 1) // Buffer of 1 to prevent blocking
	service.RegisterProgressListener(ch)

	progress := &ProgressInfo{
		FileName: "test.csv",
		Status:   "processing",
	}

	service.BroadcastProgress(progress)

	select {
	case received := <-ch:
		assert.Equal(t, progress, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for progress broadcast")
	}
}

func TestGetFileProgress(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)

	// Add a test progress
	fileName := "test.csv"
	progress := &ProgressInfo{
		FileName: fileName,
		Status:   "processing",
	}

	service.fileProgressLock.Lock()
	service.fileProgressMap[fileName] = progress
	service.fileProgressLock.Unlock()

	// Test getting existing progress
	result := service.GetFileProgress(fileName)
	assert.NotNil(t, result)
	assert.Equal(t, fileName, result.FileName)
	assert.Equal(t, "processing", result.Status)

	// Test getting non-existent progress
	result = service.GetFileProgress("nonexistent.csv")
	assert.Nil(t, result)
}

func TestGetAllFileProgress(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)

	// Add test progress entries
	progress1 := &ProgressInfo{FileName: "file1.csv", Status: "processing"}
	progress2 := &ProgressInfo{FileName: "file2.csv", Status: "completed"}

	service.fileProgressLock.Lock()
	service.fileProgressMap["file1.csv"] = progress1
	service.fileProgressMap["file2.csv"] = progress2
	service.fileProgressLock.Unlock()

	// Get all progress
	results := service.GetAllFileProgress()

	assert.Equal(t, 2, len(results))

	// Check if both progress entries are in the results
	foundFile1 := false
	foundFile2 := false

	for _, p := range results {
		if p.FileName == "file1.csv" {
			foundFile1 = true
			assert.Equal(t, "processing", p.Status)
		}
		if p.FileName == "file2.csv" {
			foundFile2 = true
			assert.Equal(t, "completed", p.Status)
		}
	}

	assert.True(t, foundFile1, "file1.csv progress not found")
	assert.True(t, foundFile2, "file2.csv progress not found")
}

func TestUpdateProgress(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)
	ch := make(chan *ProgressInfo, 1)
	service.RegisterProgressListener(ch)

	// Set up initial progress
	fileName := "test.csv"
	initialProgress := &ProgressInfo{
		FileName:  fileName,
		Processed: 10,
	}

	service.fileProgressLock.Lock()
	service.fileProgressMap[fileName] = initialProgress
	service.fileProgressLock.Unlock()

	// Update progress
	service.updateProgress(fileName, 5)

	// Check updated value
	progress := service.GetFileProgress(fileName)
	assert.Equal(t, 15, progress.Processed)

	// Check broadcast
	select {
	case received := <-ch:
		assert.Equal(t, fileName, received.FileName)
		assert.Equal(t, 15, received.Processed)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for progress broadcast")
	}
}

func TestUpdateProgressError(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)
	ch := make(chan *ProgressInfo, 1)
	service.RegisterProgressListener(ch)

	// Set up initial progress
	fileName := "test.csv"
	initialProgress := &ProgressInfo{
		FileName: fileName,
		Status:   "processing",
	}

	service.fileProgressLock.Lock()
	service.fileProgressMap[fileName] = initialProgress
	service.fileProgressLock.Unlock()

	// Update progress with error
	errorMsg := "test error"
	service.updateProgressError(fileName, errorMsg)

	// Check updated values
	progress := service.GetFileProgress(fileName)
	assert.Equal(t, "error", progress.Status)
	assert.Equal(t, errorMsg, progress.Error)
	assert.False(t, progress.EndTime.IsZero())

	// Check broadcast
	select {
	case received := <-ch:
		assert.Equal(t, fileName, received.FileName)
		assert.Equal(t, "error", received.Status)
		assert.Equal(t, errorMsg, received.Error)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for progress broadcast")
	}
}

func TestCountRecords(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)

	// Create a temporary CSV file for testing
	tempDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test.csv")
	content := "header1,header2,header3\nrow1a,row1b,row1c\nrow2a,row2b,row2c\nrow3a,row3b,row3c"
	err = ioutil.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test CSV: %v", err)
	}

	// Count records
	count, err := service.countRecords(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestSaveBatch(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)

	// Create test data
	students := []model.Student{
		{StudentID: "S001", StudentName: "Alice", Subject: "Math", Grade: 95},
		{StudentID: "S002", StudentName: "Bob", Subject: "Science", Grade: 87},
	}

	// Save batch
	service.saveBatch(students)

	// Verify saved data
	var count int64
	db.Model(&model.Student{}).Count(&count)
	assert.Equal(t, int64(2), count)

	var found model.Student
	db.Where("student_id = ?", "S001").First(&found)
	assert.Equal(t, "Alice", found.StudentName)
	assert.Equal(t, "Math", found.Subject)
	assert.Equal(t, 95, found.Grade)
}

func TestProcessCSV(t *testing.T) {
	db := setupTestDB()
	service := NewUploadService(db)

	// Create a temporary CSV file for testing
	tempDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fileName := "test.csv"
	tempFile := filepath.Join(tempDir, fileName)
	content := "StudentID,StudentName,Subject,Grade\n" +
		"S001,Alice,Math,95\n" +
		"S002,Bob,Science,87\n" +
		"S003,Charlie,History,92"

	err = ioutil.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test CSV: %v", err)
	}

	// Process the CSV
	err = service.ProcessCSV(tempFile)
	assert.NoError(t, err)

	// Wait for processing to complete
	time.Sleep(500 * time.Millisecond)

	// Check progress
	progress := service.GetFileProgress(fileName)
	assert.NotNil(t, progress)
	assert.Equal(t, "completed", progress.Status)
	assert.Equal(t, 3, progress.TotalRecords)

	// Check database
	var count int64
	db.Model(&model.Student{}).Count(&count)
	assert.Equal(t, int64(3), count)

	var students []model.Student
	db.Find(&students)
	assert.Len(t, students, 3)

	// Check specific student
	var alice model.Student
	db.Where("student_id = ?", "S001").First(&alice)
	assert.Equal(t, "Alice", alice.StudentName)
	assert.Equal(t, "Math", alice.Subject)
	assert.Equal(t, 95, alice.Grade)
}
