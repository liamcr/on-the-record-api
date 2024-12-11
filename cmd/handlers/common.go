package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func connectToDB() (*sql.DB, error) {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	connStr := fmt.Sprintf("user=%s password=%s dbname=neondb host=%s sslmode=verify-full", user, password, host)
	return sql.Open("postgres", connStr)
}

func setupCORS(w http.ResponseWriter, _ *http.Request) {
	//Allow CORS here By * or specific origin
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, POST, GET, DELETE, OPTIONS")
}

func extractUserIDFromJWTPayload(jwt string) (string, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		// This should never occur since the JWT is validated by
		// the middleware.
		return "", errors.New("invalid JWT: must have three parts")
	}

	encodedPayload := parts[1]
	marshalledPayload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", errors.New("failed to decode payload: " + err.Error())
	}

	var payload map[string]interface{}
	err = json.Unmarshal(marshalledPayload, &payload)
	if err != nil {
		return "", errors.New("failed to unmarshal JSON payload: " + err.Error())
	}

	sub, ok := payload["sub"].(string)
	if !ok {
		return "", errors.New("sub key is not present in the payload")
	}

	return translateSubToUserID(sub), nil
}

func translateSubToUserID(sub string) string {
	authSections := strings.Split(sub, "|");

	if len(authSections) != 2 {
		slog.Error("Unexpected auth0 ID format");
		return sub;
	}

	if authSections[0] == "auth0" {
		return "0" + authSections[1];
	} else if authSections[0] == "facebook" {
		return "1" + authSections[1];
	} else if authSections[0] == "google-oauth2" {
		return "2" + authSections[1];
	}

	slog.Error("Unrecognized auth provider: " + authSections[0]);
	return sub;
}