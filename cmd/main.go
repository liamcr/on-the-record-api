package main

import (
	"log"
	"net/http"
	"on-the-record-api/cmd/handlers"
)

func main() {
	handlers.RegisterHandlers()
	
	err := http.ListenAndServe(":8080", nil);
	if err != nil {
		log.Fatal("HTTP server failed to start")
	}
}