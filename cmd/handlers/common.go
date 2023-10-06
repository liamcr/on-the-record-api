package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
)

func connectToDB() (*sql.DB, error) {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	connStr := fmt.Sprintf("user=%s password=%s dbname=neondb host=ep-twilight-butterfly-21715046.us-east-2.aws.neon.tech sslmode=verify-full", user, password)
	return sql.Open("postgres", connStr)
}

func setupCORS(w http.ResponseWriter, req *http.Request) {
	//Allow CORS here By * or specific origin
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
