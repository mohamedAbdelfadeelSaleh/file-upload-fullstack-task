// //package config
// //
// //const (
// //	DBHost     = "localhost"
// //	DBUser     = "yoru"
// //	DBPassword = "newpassword"
// //	DBName     = "studentdb"
// //	DBPort     = "5432"
// //)
//
// package config
//
// import (
//
//	"os"
//
// )
//
// var (
//
//	DBHost     = os.Getenv("DB_HOST")
//	DBUser     = os.Getenv("DB_USER")
//	DBPassword = os.Getenv("DB_PASSWORD")
//	DBName     = os.Getenv("DB_NAME")
//	DBPort     = os.Getenv("DB_PORT")
//
// )
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var (
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string
)

func LoadConfig() error {
	// Load .env file in development
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	// Validate required environment variables
	required := []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_PORT"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			return fmt.Errorf("missing required environment variable: %s", key)
		}
	}

	// Assign environment variables
	DBHost = os.Getenv("DB_HOST")
	DBUser = os.Getenv("DB_USER")
	DBPassword = os.Getenv("DB_PASSWORD")
	DBName = os.Getenv("DB_NAME")
	DBPort = os.Getenv("DB_PORT")

	return nil
}
