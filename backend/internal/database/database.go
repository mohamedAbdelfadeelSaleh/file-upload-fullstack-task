package database

import (
	"backend/internal/config"
	"backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func InitDB() *gorm.DB {
	dsn := "host=" + config.DBHost + " user=" + config.DBUser + " password=" + config.DBPassword + " dbname=" + config.DBName + " port=" + config.DBPort + " sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Auto-migrate the Student table
	if err := db.AutoMigrate(&model.Student{}); err != nil {
		log.Fatal("Failed to auto-migrate the database:", err)
	}

	return db
}
