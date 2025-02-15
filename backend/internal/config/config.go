//package config
//
//const (
//	DBHost     = "localhost"
//	DBUser     = "yoru"
//	DBPassword = "newpassword"
//	DBName     = "studentdb"
//	DBPort     = "5432"
//)

package config

import "os"

var (
	DBHost     = os.Getenv("DB_HOST")
	DBUser     = os.Getenv("DB_USER")
	DBPassword = os.Getenv("DB_PASSWORD")
	DBName     = os.Getenv("DB_NAME")
	DBPort     = os.Getenv("DB_PORT")
)
