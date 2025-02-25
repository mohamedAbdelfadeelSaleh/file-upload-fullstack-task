package database

import (
	"backend/internal/config"
	"backend/internal/model"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	db.Logger = logger.Default.LogMode(logger.Silent)

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

func TruncateAllTables(db *gorm.DB) error {
	tables := []string{"students"} // Add all table names here

	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;", table)).Error; err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}
	return nil
}
