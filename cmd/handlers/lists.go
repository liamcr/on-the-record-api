package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type ListElement struct {
	EntityID string `json:"entityId"`
	Name     string `json:"name"`
	ImageSrc string `json:"src"`
}

type addListParams struct {
	Type         int           `json:"type"`
	Title        string        `json:"title"`
	Colour       string        `json:"colour"`
	ListElements []ListElement `json:"listElements"`
}

type likeListParams struct {
	ListID string `json:"listId"`
}

type List struct {
	UserID       string        `json:"userId"`
	Type         int           `json:"type"`
	Title        string        `json:"title"`
	Colour       string        `json:"colour"`
	ListElements []ListElement `json:"listElements"`
	CreatedOn    time.Time     `json:"createdOn"`
}

func addList(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	token := r.Header["Authorization"][0][len("Bearer: "):]
	userID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	var addListBody addListParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&addListBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	if len(addListBody.ListElements) != 5 {
		slog.Error("list must have five elements")
		http.Error(w, "List must have five elements", http.StatusBadRequest)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "INSERT INTO lists (id, user_id, type, title, colour, created_on) VALUES ($1, $2, $3, $4, $5, $6);"

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	id := uuid.NewString()
	createdOn := time.Now().UTC()
	_, err = stmt.Exec(
		id,
		userID,
		addListBody.Type,
		addListBody.Title,
		addListBody.Colour,
		createdOn,
	)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add list", http.StatusInternalServerError)
		return
	}

	query = "INSERT INTO list_elements (list_id, user_id, entity_id, title, image_src, placement) VALUES ($1, $2, $3, $4, $5, 1), ($1, $2, $6, $7, $8, 2), ($1, $2, $9, $10, $11, 3), ($1, $2, $12, $13, $14, 4), ($1, $2, $15, $16, $17, 5);"

	stmt, err = tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(
		id,
		userID,
		addListBody.ListElements[0].EntityID,
		addListBody.ListElements[0].Name,
		addListBody.ListElements[0].ImageSrc,
		addListBody.ListElements[1].EntityID,
		addListBody.ListElements[1].Name,
		addListBody.ListElements[1].ImageSrc,
		addListBody.ListElements[2].EntityID,
		addListBody.ListElements[2].Name,
		addListBody.ListElements[2].ImageSrc,
		addListBody.ListElements[3].EntityID,
		addListBody.ListElements[3].Name,
		addListBody.ListElements[3].ImageSrc,
		addListBody.ListElements[4].EntityID,
		addListBody.ListElements[4].Name,
		addListBody.ListElements[4].ImageSrc,
	)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add list elements", http.StatusInternalServerError)
		return
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add lists", http.StatusInternalServerError)
		return
	}

	resp := List{
		UserID:       userID,
		Title:        addListBody.Title,
		Colour:       addListBody.Colour,
		ListElements: addListBody.ListElements,
		CreatedOn:    createdOn,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func deleteList(w http.ResponseWriter, r *http.Request) {
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

	token := r.Header["Authorization"][0][len("Bearer: "):]
	currentUserID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	getListUserQuery := "SELECT user_id FROM lists WHERE id = $1"

	var listUserID string
	err = db.QueryRow(getListUserQuery, id).Scan(&listUserID)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to delete list", http.StatusInternalServerError)
		return
	}
	if currentUserID != listUserID {
		slog.Error(
			"user does not have permission to delete this list",
			"requestingId", currentUserID,
			"reviewUserId", listUserID,
		)
		http.Error(w, "user cannot delete someone else's list", http.StatusForbidden)
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "DELETE FROM list_elements WHERE list_id = $1;"

	_, err = tx.Exec(query, id)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete list", "error", err)
		http.Error(w, "Failed to delete list", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM lists WHERE id = $1;"

	_, err = tx.Exec(query, id)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete list", "error", err)
		http.Error(w, "Failed to delete list", http.StatusInternalServerError)
		return
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add lists", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func likeList(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	token := r.Header["Authorization"][0][len("Bearer: "):]
	userID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	var likeListBody likeListParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&likeListBody); err != nil {
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

	// Make sure user hasn't already liked this list
	checkIfUserHasLikedQuery := "SELECT COUNT(*) FROM list_likes WHERE user_id = $1 AND list_id = $2"

	var count int
	err = db.QueryRow(checkIfUserHasLikedQuery, userID, likeListBody.ListID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		slog.Error("this user already has already liked this list", "User ID", userID, "List ID", likeListBody.ListID)
		http.Error(w, "this user already has already liked this list", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO list_likes (list_id, user_id) VALUES ($1, $2)"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(
		likeListBody.ListID,
		userID,
	)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to like list", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func unlikeList(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	token := r.Header["Authorization"][0][len("Bearer: "):]
	userID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	var likeListBody likeListParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&likeListBody); err != nil {
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

	// Make sure user hasn't already liked this list
	checkIfUserHasLikedQuery := "SELECT COUNT(*) FROM list_likes WHERE user_id = $1 AND list_id = $2"

	var count int
	err = db.QueryRow(checkIfUserHasLikedQuery, userID, likeListBody.ListID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		slog.Error("this user has not liked this list", "User ID", userID, "List ID", likeListBody.ListID)
		http.Error(w, "this user already has not liked this list", http.StatusBadRequest)
		return
	}

	query := "DELETE FROM list_likes WHERE list_id = $1 AND user_id = $2;"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(
		likeListBody.ListID,
		userID,
	)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to like list", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func getListLikes(w http.ResponseWriter, r *http.Request) {
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

	query := "SELECT u.id, u.name, u.image_src FROM list_likes l JOIN users u ON l.user_id = u.id WHERE l.list_id = $1"

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
