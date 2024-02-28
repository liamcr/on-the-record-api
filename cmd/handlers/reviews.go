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
	UserID      string `json:"userId"`
	EntityID    string `json:"entityId"`
	Type        int    `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	ImageSource string `json:"imageSrc"`
	Score       int    `json:"score"`
	Body        string `json:"body"`
}

type likeReviewParams struct {
	UserID   string `json:"userId"`
	ReviewID int    `json:"reviewId"`
}

type Review struct {
	UserID      string    `json:"userId"`
	EntityID    string    `json:"entityId"`
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

	query := "INSERT INTO reviews (user_id, entity_id, type, title, subtitle, colour, image_src, score, body, created_on) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	createdOn := time.Now().UTC()
	_, err = stmt.Exec(
		addReviewBody.UserID,
		addReviewBody.EntityID,
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
		http.Error(w, "Failed to add review", http.StatusInternalServerError)
		return
	}

	resp := Review{
		UserID:      addReviewBody.UserID,
		EntityID:    addReviewBody.EntityID,
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
		slog.Error("could not delete review", "error", err)
		http.Error(w, "Failed to delete review", http.StatusInternalServerError)
		return
	}

	if err != nil {
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add review", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func likeReview(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	var likeReviewBody likeReviewParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&likeReviewBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user hasn't already liked this review
	checkIfUserHasLikedQuery := "SELECT COUNT(*) FROM review_likes WHERE user_id = $1 AND review_id = $2"

	var count int
	err = db.QueryRow(checkIfUserHasLikedQuery, likeReviewBody.UserID, likeReviewBody.ReviewID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		slog.Error("this user already has already liked this review", "User ID", likeReviewBody.UserID, "Review ID", likeReviewBody.ReviewID)
		http.Error(w, "this user already has already liked this review", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO review_likes (review_id, user_id) VALUES ($1, $2)"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(
		likeReviewBody.ReviewID,
		likeReviewBody.UserID,
	)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to like review", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func unlikeReview(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	var likeReviewBody likeReviewParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&likeReviewBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user hasn't already liked this review
	checkIfUserHasLikedQuery := "SELECT COUNT(*) FROM review_likes WHERE user_id = $1 AND review_id = $2"

	var count int
	err = db.QueryRow(checkIfUserHasLikedQuery, likeReviewBody.UserID, likeReviewBody.ReviewID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		slog.Error("this user already has not liked this review", "User ID", likeReviewBody.UserID, "Review ID", likeReviewBody.ReviewID)
		http.Error(w, "this user already has not liked this review", http.StatusBadRequest)
		return
	}

	query := "DELETE FROM review_likes WHERE review_id = $1 AND user_id = $2;"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(
		likeReviewBody.ReviewID,
		likeReviewBody.UserID,
	)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to like review", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func getReviewLikes(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	ID := r.URL.Query().Get("id")
	if ID == "" {
		http.Error(w, "Missing query param: id", http.StatusBadRequest)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := "SELECT u.id, u.name, u.image_src FROM review_likes l JOIN users u ON l.user_id = u.id WHERE l.review_id = $1"

	likeRows, err := db.Query(query, ID)
	if err != nil {
		slog.Error("could not get likes", "error", err)
		http.Error(w, "Failed to get likes", http.StatusInternalServerError)
		return
	}
	defer likeRows.Close()

	usersThatLiked := []UserCondensed{}
	for likeRows.Next() {
		var user UserCondensed
		if err := likeRows.Scan(&user.ID, &user.Name, &user.ImageSource); err != nil {
			slog.Error("failed to scan row", "error", err)
			http.Error(w, "Could not get likes", http.StatusInternalServerError)
			return
		}
		usersThatLiked = append(usersThatLiked, user)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usersThatLiked)
}
