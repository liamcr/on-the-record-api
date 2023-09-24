package handlers

import (
	"database/sql"
	"fmt"
	"os"
)

func connectToDB() (*sql.DB, error) {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	connStr := fmt.Sprintf("user=%s password=%s dbname=neondb host=ep-twilight-butterfly-21715046.us-east-2.aws.neon.tech sslmode=verify-full", user, password)
	return sql.Open("postgres", connStr)
}
