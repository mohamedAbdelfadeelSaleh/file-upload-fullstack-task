package database

import (
	"backend/internal/config"
	"backend/internal/model"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"time"
)

func InitDB() *gorm.DB {
	// Ensure config is loaded
	if config.DBHost == "" || config.DBUser == "" || config.DBPassword == "" || config.DBName == "" || config.DBPort == "" {
		log.Fatal("Database configuration is missing or incomplete")
	}

	// Construct DSN using fmt.Sprintf
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		config.DBHost, config.DBUser, config.DBPassword, config.DBName, config.DBPort)

	// Open database connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Configure connection pooling
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database instance:", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Auto-migrate the Student table
	if err := db.AutoMigrate(&model.Student{}); err != nil {
		log.Fatal("Failed to auto-migrate the database:", err)
	}

	return db
}
