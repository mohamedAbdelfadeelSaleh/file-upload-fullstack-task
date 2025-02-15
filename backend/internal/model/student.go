package model

type Student struct {
	StudentID   string `gorm:"primaryKey"` // StudentID is the primary key
	StudentName string
	Subject     string
	Grade       int
}
