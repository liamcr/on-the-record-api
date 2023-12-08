package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"on-the-record-api/cmd/util"
	"time"

	_ "github.com/lib/pq"
)

type addReviewParams struct {
	Provider    string `json:"provider"`
	ProviderID  string `json:"providerId"`
	Type        int    `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	ImageSource string `json:"imageSrc"`
	Score       int    `json:"score"`
	Body        string `json:"body"`
}

type Review struct {
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"providerId"`
	Type        int       `json:"type"`
	Title       string    `json:"title"`
	Subtitle    string    `json:"subtitle"`
	Colour      string    `json:"colour"`
	ImageSource string    `json:"imageSrc"`
	Score       int       `json:"score"`
	Body        string    `json:"body"`
	CreatedOn   time.Time `json:"createdOn"`
}

func addReview(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	var addReviewBody addReviewParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&addReviewBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	dominantColour, err := util.GetDominantColourFromImage(addReviewBody.ImageSource)
	if err != nil {
		slog.Error("could not get colour from image", "error", err)
		http.Error(w, "Failed to get colour from image", http.StatusInternalServerError)
		return
	}
	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := "INSERT INTO reviews (user_provider, user_provider_id, type, title, subtitle, colour, image_src, score, body, created_on) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	createdOn := time.Now()
	_, err = stmt.Exec(
		addReviewBody.Provider,
		addReviewBody.ProviderID,
		addReviewBody.Type,
		addReviewBody.Title,
		addReviewBody.Subtitle,
		dominantColour,
		addReviewBody.ImageSource,
		addReviewBody.Score,
		addReviewBody.Body,
		createdOn,
	)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	resp := Review{
		Provider:    addReviewBody.Provider,
		ProviderID:  addReviewBody.ProviderID,
		Title:       addReviewBody.Title,
		Subtitle:    addReviewBody.Subtitle,
		Colour:      dominantColour,
		ImageSource: addReviewBody.ImageSource,
		Score:       addReviewBody.Score,
		Body:        addReviewBody.Body,
		CreatedOn:   createdOn,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func deleteReview(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing query params: id", http.StatusBadRequest)
		return
	}

	query := "DELETE FROM reviews WHERE id = $1;"

	_, err = db.Exec(query, id)
	if err != nil {
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	if err != nil {
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}
