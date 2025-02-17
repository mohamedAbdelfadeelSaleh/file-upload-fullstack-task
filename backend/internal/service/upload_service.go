package service

import (
	"backend/internal/model"
	"encoding/csv"
	"gorm.io/gorm"
	"io"
	"log"
	//"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type ProgressInfo struct {
	FileName     string
	TotalRecords int
	Processed    int
	Status       string // "processing", "completed", "error"
	Error        string
	StartTime    time.Time
	EndTime      time.Time
}

type UploadService struct {
	db                *gorm.DB
	fileProgressMap   map[string]*ProgressInfo
	fileProgressLock  sync.RWMutex
	progressListeners map[chan *ProgressInfo]bool // Track SSE listeners
	listenerLock      sync.RWMutex

	////////////////////////////////////
	workerSemaphore      chan struct{} // Semaphore to limit total workers
	maxConcurrentWorkers int
}

func NewUploadService(db *gorm.DB) *UploadService {
	maxWorkers := runtime.NumCPU() * 2 // Reasonable default

	return &UploadService{
		db:                   db,
		fileProgressMap:      make(map[string]*ProgressInfo),
		progressListeners:    make(map[chan *ProgressInfo]bool),
		workerSemaphore:      make(chan struct{}, maxWorkers),
		maxConcurrentWorkers: maxWorkers,
	}
}

func (s *UploadService) RegisterProgressListener(ch chan *ProgressInfo) {
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	s.progressListeners[ch] = true
}

// UnregisterProgressListener removes a client from receiving progress updates
func (s *UploadService) UnregisterProgressListener(ch chan *ProgressInfo) {
	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()
	delete(s.progressListeners, ch)
}

// BroadcastProgress sends progress updates to all registered listeners
func (s *UploadService) BroadcastProgress(progress *ProgressInfo) {
	s.listenerLock.RLock()
	defer s.listenerLock.RUnlock()

	for listener := range s.progressListeners {
		select {
		case listener <- progress:
		default:
			// Skip if the listener is not ready
		}
	}
}

func (s *UploadService) updateProgress(fileName string, processed int) {
	s.fileProgressLock.Lock()
	defer s.fileProgressLock.Unlock()

	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Processed += processed
		// Ensure that Processed does not exceed TotalRecords
		if progress.Processed > progress.TotalRecords {
			progress.Processed = progress.TotalRecords
		}
		s.BroadcastProgress(progress)
	}
}

// Update progress with error and broadcast to listeners
func (s *UploadService) updateProgressError(fileName string, errorMsg string) {
	s.fileProgressLock.Lock()
	defer s.fileProgressLock.Unlock()

	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Status = "error"
		progress.Error = errorMsg
		progress.EndTime = time.Now()
		s.BroadcastProgress(progress)
	}
}

////////////////////////////////////////////////////////

func (s *UploadService) GetFileProgress(fileName string) *ProgressInfo {
	s.fileProgressLock.RLock()
	defer s.fileProgressLock.RUnlock()

	if progress, exists := s.fileProgressMap[fileName]; exists {
		// Return a copy to avoid race conditions
		copyProgress := *progress
		return &copyProgress
	}

	return nil
}

func (s *UploadService) GetAllFileProgress() []*ProgressInfo {
	s.fileProgressLock.RLock()
	defer s.fileProgressLock.RUnlock()

	result := make([]*ProgressInfo, 0, len(s.fileProgressMap))
	for _, progress := range s.fileProgressMap {
		// Create a copy to avoid race conditions
		copyProgress := *progress
		result = append(result, &copyProgress)
	}

	return result
}

func (s *UploadService) ProcessCSV(filePath string) error {
	fileName := filepath.Base(filePath)
	startTime := time.Now()

	// Initialize progress tracking
	s.fileProgressLock.Lock()
	s.fileProgressMap[fileName] = &ProgressInfo{
		FileName:     fileName,
		TotalRecords: 0,
		Processed:    0,
		Status:       "processing",
		StartTime:    startTime,
	}
	s.fileProgressLock.Unlock()

	// Get file info for size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		s.updateProgressError(fileName, "Failed to get file info: "+err.Error())
		return err
	}

	// Calculate number of workers based on file size
	numWorkers := calculateWorkers(fileInfo.Size())
	log.Printf("Using %d workers for file %s (size: %d bytes)\n", numWorkers, fileName, fileInfo.Size())

	file, err := os.Open(filePath)
	if err != nil {
		s.updateProgressError(fileName, "Failed to open file: "+err.Error())
		return err
	}
	defer file.Close()

	// Count total records
	totalRecords, err := s.countRecords(filePath)
	if err != nil {
		s.updateProgressError(fileName, "Failed to count records: "+err.Error())
		return err
	}

	s.fileProgressLock.Lock()
	s.fileProgressMap[fileName].TotalRecords = totalRecords
	s.fileProgressLock.Unlock()

	// Reopen the file for actual processing
	file.Seek(0, 0)
	reader := csv.NewReader(file)
	reader.Read() // Skip header row

	// Buffer size based on number of workers
	bufferSize := 1000
	if numWorkers > 10 {
		bufferSize = numWorkers * 100
	}

	studentCh := make(chan []string, bufferSize)
	var wg sync.WaitGroup
	existingIDs := sync.Map{}

	// Launch workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go s.worker(fileName, studentCh, &existingIDs, &wg)
	}

	// Read records and send them to workers
	go func() {
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println("Error reading CSV record:", err)
				continue
			}
			studentCh <- record
		}
		close(studentCh) // Close the channel after all records are read
	}()

	// Wait for all workers to finish
	wg.Wait()

	// Update progress as completed
	s.fileProgressLock.Lock()
	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Status = "completed"
		progress.EndTime = time.Now()
		progress.Processed = progress.TotalRecords // Ensure processed equals total records
		s.BroadcastProgress(progress)
	}
	s.fileProgressLock.Unlock()

	// Log processing completion
	log.Printf("Processing completed for %s in %v\n", fileName, time.Since(startTime))

	return nil
}

// calculateWorkers determines the appropriate number of workers based on file size
func calculateWorkers(fileSize int64) int {
	// Base calculations on available CPUs
	cpus := runtime.NumCPU()

	// For very small files (< 1MB), use just 2 workers
	if fileSize < 1_000_000 {
		return min(2, cpus)
	}

	// For small files (1-10MB), use up to 4 workers
	if fileSize < 10_000_000 {
		return min(4, cpus)
	}

	// For medium files (10-100MB), scale up to 8 workers
	if fileSize < 100_000_000 {
		return min(8, cpus)
	}

	// For large files (100MB-1GB), scale up to 16 workers
	if fileSize < 1_000_000_000 {
		return min(16, cpus)
	}

	// For very large files (>1GB), use all available CPUs
	return cpus
}

func (s *UploadService) worker(fileName string, studentCh chan []string, existingIDs *sync.Map, wg *sync.WaitGroup) {
	s.workerSemaphore <- struct{}{}
	defer func() {
		// Release semaphore
		<-s.workerSemaphore
		wg.Done() // Only call wg.Done() once here
	}()

	// Remove this duplicate defer wg.Done()

	var students []model.Student
	processedCount := 0

	for record := range studentCh {
		studentID := record[0]
		if _, exists := existingIDs.Load(studentID); exists {
			log.Printf("Skipping duplicate student ID: %s\n", studentID)
			continue
		}

		grade, err := strconv.Atoi(record[3])
		if err != nil {
			log.Println("Error converting grade to integer:", err)
			continue
		}

		students = append(students, model.Student{
			StudentID:   studentID,
			StudentName: record[1],
			Subject:     record[2],
			Grade:       grade,
		})

		existingIDs.Store(studentID, true)
		processedCount++

		// Update progress periodically
		if processedCount%100 == 0 {
			s.updateProgress(fileName, processedCount)
		}

		if len(students) >= 1000 {
			s.saveBatch(students)
			students = nil
		}
	}

	if len(students) > 0 {
		s.saveBatch(students)
	}

	// Final progress update for this worker
	s.updateProgress(fileName, processedCount)
}

func (s *UploadService) countRecords(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Read() // Skip header

	count := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

func (s *UploadService) saveBatch(students []model.Student) {
	if len(students) == 0 {
		return
	}

	var values []interface{}
	query := "INSERT INTO students (student_id, student_name, subject, grade) VALUES "

	for i, student := range students {
		if i > 0 {
			query += ","
		}
		query += "(?, ?, ?, ?)"
		values = append(values, student.StudentID, student.StudentName, student.Subject, student.Grade)
	}

	query += " ON CONFLICT (student_id) DO NOTHING"

	err := s.db.Exec(query, values...).Error

	if err != nil {
		//log.Println("Error inserting batch into database:", err)
	} else {
		//log.Printf("Inserted %d students into database\n", len(students))
	}
}
