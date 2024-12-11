package main

import (
	"fmt"
	"log"
	"net/http"
	"on-the-record-api/cmd/handlers"

	"github.com/joho/godotenv"
)

const port = 8080

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	handlers.RegisterHandlers()

	fmt.Printf("Listening on port %d...\n", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("HTTP server failed to start")
	}
}
