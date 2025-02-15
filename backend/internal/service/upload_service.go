package service

import (
	"backend/internal/model"
	"encoding/csv"
	"gorm.io/gorm"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type UploadService struct {
	db *gorm.DB
}

func NewUploadService(db *gorm.DB) *UploadService {
	return &UploadService{db: db}
}

func (s *UploadService) ProcessCSV(filePath string) error {
	startTime := time.Now()

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Read() // Skip header row

	studentCh := make(chan []string, 1000)
	var wg sync.WaitGroup
	existingIDs := sync.Map{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go s.worker(studentCh, &existingIDs, &wg)
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

	log.Printf("Processing completed in %v\n", time.Since(startTime))
	return nil
}

func (s *UploadService) worker(studentCh chan []string, existingIDs *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

	var students []model.Student

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

		if len(students) >= 1000 {
			s.saveBatch(students)
			students = nil
		}
	}

	if len(students) > 0 {
		s.saveBatch(students)
	}
}

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
