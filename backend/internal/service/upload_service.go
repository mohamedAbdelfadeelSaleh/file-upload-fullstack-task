package service

import (
	"backend/internal/model"
	"encoding/csv"
	"gorm.io/gorm"
	"io"
	"log"
	"os"
	"path/filepath"
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

//	type UploadService struct {
//		db *gorm.DB
//	}
type UploadService struct {
	db               *gorm.DB
	fileProgressMap  map[string]*ProgressInfo
	fileProgressLock sync.RWMutex
}

//	func NewUploadService(db *gorm.DB) *UploadService {
//		return &UploadService{db: db}
//	}
func NewUploadService(db *gorm.DB) *UploadService {
	return &UploadService{
		db:              db,
		fileProgressMap: make(map[string]*ProgressInfo),
	}
}

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

//func (s *UploadService) ProcessCSV(filePath string) error {
//	startTime := time.Now()
//
//	file, err := os.Open(filePath)
//	if err != nil {
//		return err
//	}
//	defer file.Close()
//
//	reader := csv.NewReader(file)
//	reader.Read() // Skip header row
//
//	studentCh := make(chan []string, 1000)
//	var wg sync.WaitGroup
//	existingIDs := sync.Map{}
//
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go s.worker(studentCh, &existingIDs, &wg)
//	}
//
//	for {
//		record, err := reader.Read()
//		if err == io.EOF {
//			break
//		}
//		if err != nil {
//			log.Println("Error reading CSV record:", err)
//			continue
//		}
//		studentCh <- record
//	}
//
//	close(studentCh)
//	wg.Wait()
//
//	log.Printf("Processing completed in %v\n", time.Since(startTime))
//	return nil
//}

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

	studentCh := make(chan []string, 1000)
	var wg sync.WaitGroup
	existingIDs := sync.Map{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go s.worker(fileName, studentCh, &existingIDs, &wg)
	}

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

	close(studentCh)
	wg.Wait()

	// Update progress as completed
	s.fileProgressLock.Lock()
	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Status = "completed"
		progress.EndTime = time.Now()
	}
	s.fileProgressLock.Unlock()

	log.Printf("Processing completed for %s in %v\n", fileName, time.Since(startTime))
	return nil
}

//func (s *UploadService) worker(studentCh chan []string, existingIDs *sync.Map, wg *sync.WaitGroup) {
//	defer wg.Done()
//
//	var students []model.Student
//
//	for record := range studentCh {
//		studentID := record[0]
//		if _, exists := existingIDs.Load(studentID); exists {
//			log.Printf("Skipping duplicate student ID: %s\n", studentID)
//			continue
//		}
//
//		grade, err := strconv.Atoi(record[3])
//		if err != nil {
//			log.Println("Error converting grade to integer:", err)
//			continue
//		}
//
//		students = append(students, model.Student{
//			StudentID:   studentID,
//			StudentName: record[1],
//			Subject:     record[2],
//			Grade:       grade,
//		})
//
//		existingIDs.Store(studentID, true)
//
//		if len(students) >= 1000 {
//			s.saveBatch(students)
//			students = nil
//		}
//	}
//
//	if len(students) > 0 {
//		s.saveBatch(students)
//	}
//}

func (s *UploadService) worker(fileName string, studentCh chan []string, existingIDs *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

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

func (s *UploadService) updateProgress(fileName string, processed int) {
	s.fileProgressLock.Lock()
	defer s.fileProgressLock.Unlock()

	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Processed += processed
	}
}

func (s *UploadService) updateProgressError(fileName string, errorMsg string) {
	s.fileProgressLock.Lock()
	defer s.fileProgressLock.Unlock()

	if progress, exists := s.fileProgressMap[fileName]; exists {
		progress.Status = "error"
		progress.Error = errorMsg
		progress.EndTime = time.Now()
	}
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

//
//func (s *UploadService) saveBatch(students []model.Student) {
//	if len(students) == 0 {
//		return
//	}
//
//	err := s.db.Transaction(func(tx *gorm.DB) error {
//		return tx.Create(&students).Error
//	})
//	if err != nil {
//		log.Println("Error inserting batch into database:", err)
//	} else {
//		log.Printf("Inserted %d students into database\n", len(students))
//	}
//}

func (s *UploadService) saveBatch(students []model.Student) {
	if len(students) == 0 {
		return
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&students).Error
	})
	if err != nil {
		log.Println("Error inserting batch into database:", err)
	} else {
		log.Printf("Inserted %d students into database\n", len(students))
	}
}
