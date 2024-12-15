package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type addUserParams struct {
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	MusicNotes  []MusicNote `json:"musicNotes"`
}

type updateUserParams struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	MusicNotes  []MusicNote `json:"musicNotes"`
}

type followUserParams struct {
	ID string `json:"id"`
}

type User struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	Followers   int         `json:"followers"`
	Following   int         `json:"following"`
	Reviews     int         `json:"reviews"`
	Lists       int         `json:"lists"`
	IsFollowing bool        `json:"isFollowing"`
	MusicNotes  []MusicNote `json:"musicNotes"`
	CreatedOn   time.Time   `json:"createdOn"`
}

type UserCondensed struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ImageSource string `json:"imageSrc"`
}

type MusicNote struct {
	EntityID    string `json:"entityId"`
	Prompt      string `json:"prompt"`
	ImageSource string `json:"imageSrc"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
}

func getUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	ID := r.URL.Query().Get("id")
	requestingID := r.URL.Query().Get("requesting_id")
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

	selectStatement := "SELECT id, name, colour, image_src, created_on FROM users WHERE id = $1"
	rows, err := db.Query(selectStatement, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Colour, &user.ImageSource, &user.CreatedOn); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		slog.Error("could not find user", "id", ID)
		http.Error(w, "Couldn't find user", http.StatusNotFound)
		return
	}
	if len(users) > 1 {
		slog.Error("expected 1 user, found more", "matching_users", len(users), "id", ID)
		http.Error(w, "found too many matching users", http.StatusInternalServerError)
		return
	}

	numFollowersStatement := "SELECT count(*) FROM follower_relation WHERE followee_id = $1"
	rows, err = db.Query(numFollowersStatement, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&users[0].Followers); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
	}

	numFollowingStatement := "SELECT count(*) FROM follower_relation WHERE follower_id = $1"
	rows, err = db.Query(numFollowingStatement, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&users[0].Following); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
	}

	numRows := 0
	if requestingID != "" {
		isCurrentUserFollowingQuery := "SELECT count(*) FROM follower_relation WHERE follower_id = $1 AND followee_id = $2"
		rows, err = db.Query(isCurrentUserFollowingQuery, requestingID, ID)
		if err != nil {
			slog.Error("could not get user", "error", err)
			http.Error(w, "Failed to get user", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			if err := rows.Scan(&numRows); err != nil {
				http.Error(w, "Failed to scan row", http.StatusInternalServerError)
				return
			}
		}
	}

	users[0].IsFollowing = numRows > 0

	numReviewsStatement := "SELECT count(*) FROM reviews WHERE user_id = $1"
	rows, err = db.Query(numReviewsStatement, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&users[0].Reviews); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
	}

	numListsStatement := "SELECT count(*) FROM lists WHERE user_id = $1"
	rows, err = db.Query(numListsStatement, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&users[0].Lists); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
	}

	query := "SELECT entity_id, prompt, image_src, title, subtitle FROM music_notes WHERE user_id = $1;"
	rows, err = db.Query(query, ID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	musicNotes := []MusicNote{}
	for rows.Next() {
		var musicNote MusicNote
		if err := rows.Scan(&musicNote.EntityID, &musicNote.Prompt, &musicNote.ImageSource, &musicNote.Title, &musicNote.Subtitle); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		musicNotes = append(musicNotes, musicNote)
	}

	users[0].MusicNotes = musicNotes

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users[0])
}

var featuredUsers = []followUserParams{
	{
		ID: "2114595843372308472544",
	},
}

func getFeaturedUsers(w http.ResponseWriter, r *http.Request) {
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

	whereClause := "(id = $1)"
	params := []any{featuredUsers[0].ID}
	for i, user := range featuredUsers {
		if i == 0 {
			continue
		}

		whereClause += fmt.Sprintf(" OR (id = $%d)", i+1)
		params = append(params, user.ID)
	}

	selectStatement := "SELECT id, name, image_src FROM users WHERE " + whereClause
	rows, err := db.Query(selectStatement, params...)
	if err != nil {
		slog.Error("could not get featured users", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []UserCondensed
	for rows.Next() {
		var user UserCondensed
		if err := rows.Scan(&user.ID, &user.Name, &user.ImageSource); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		slog.Error("could not find featured users")
		http.Error(w, "Couldn't find user", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func addUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	var addUserBody addUserParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&addUserBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	token := r.Header["Authorization"][0][len("Bearer: "):]
	newUserID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user doesn't already have an account
	checkIfUserExistsQuery := "SELECT COUNT(*) FROM users WHERE id = $1"

	var count int
	err = db.QueryRow(checkIfUserExistsQuery, newUserID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		slog.Error("this user already has an account", "id", newUserID)
		http.Error(w, "this user already has an account", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "INSERT INTO users (id, name, colour, image_src, created_on) VALUES ($1, $2, $3, $4, $5);"

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	createdOn := time.Now()
	_, err = stmt.Exec(newUserID, addUserBody.Name, addUserBody.Colour, addUserBody.ImageSource, createdOn)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	insertMusicNoteQuery := "INSERT INTO music_notes (user_id, entity_id, prompt, image_src, title, subtitle) VALUES ($1, $2, $3, $4, $5, $6);"
	for _, musicNote := range addUserBody.MusicNotes {
		stmt, err := tx.Prepare(insertMusicNoteQuery)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to prepare SQL statement", "error", err)
			http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(newUserID, musicNote.EntityID, musicNote.Prompt, musicNote.ImageSource, musicNote.Title, musicNote.Subtitle)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to execute SQL statement", "error", err)
			http.Error(w, "Failed to add user", http.StatusInternalServerError)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	resp := User{
		ID:          newUserID,
		Name:        addUserBody.Name,
		ImageSource: addUserBody.ImageSource,
		Colour:      addUserBody.Colour,
		MusicNotes:  addUserBody.MusicNotes,
		CreatedOn:   createdOn,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	var updateUserBody updateUserParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updateUserBody); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("failed to close request body", "error", err)
		}
	}()

	token := r.Header["Authorization"][0][len("Bearer: "):]
	currentUserID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}
	if currentUserID != updateUserBody.ID {
		http.Error(w, "Can only update your own user", http.StatusForbidden)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user exists
	checkIfUserExistsQuery := "SELECT COUNT(*) FROM users WHERE id = $1"

	var count int
	err = db.QueryRow(checkIfUserExistsQuery, updateUserBody.ID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		slog.Error("could not find user", "id", updateUserBody.ID)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "UPDATE users SET name = $2, colour = $3, image_src = $4 WHERE id = $1;"

	_, err = tx.Exec(query, updateUserBody.ID, updateUserBody.Name, updateUserBody.Colour, updateUserBody.ImageSource)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	// Delete existing music notes in order to overwrite them with the new ones
	query = "DELETE FROM music_notes WHERE user_id = $1"

	_, err = tx.Exec(query, updateUserBody.ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	insertMusicNoteQuery := "INSERT INTO music_notes (user_id, entity_id, prompt, image_src, title, subtitle) VALUES ($1, $2, $3, $4, $5, $6);"
	for _, musicNote := range updateUserBody.MusicNotes {
		stmt, err := tx.Prepare(insertMusicNoteQuery)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to prepare SQL statement", "error", err)
			http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(updateUserBody.ID, musicNote.EntityID, musicNote.Prompt, musicNote.ImageSource, musicNote.Title, musicNote.Subtitle)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to execute SQL statement", "error", err)
			http.Error(w, "Failed to add user", http.StatusInternalServerError)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	resp := updateUserParams{
		ID:          updateUserBody.ID,
		Name:        updateUserBody.Name,
		ImageSource: updateUserBody.ImageSource,
		Colour:      updateUserBody.Colour,
		MusicNotes:  updateUserBody.MusicNotes,
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
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

	ID := r.URL.Query().Get("id")
	if ID == "" {
		http.Error(w, "Missing query params: id", http.StatusBadRequest)
		return
	}

	token := r.Header["Authorization"][0][len("Bearer: "):]
	currentUserID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}
	if currentUserID != ID {
		http.Error(w, "Can only delete your own user", http.StatusForbidden)
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "DELETE FROM music_notes WHERE user_id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM list_elements WHERE user_id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM lists WHERE user_id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM reviews WHERE user_id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM follower_relation WHERE follower_id = $1 OR followee_id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM users WHERE id = $1"

	_, err = tx.Exec(query, ID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		slog.Error("failed to commit transaction", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func searchUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing query param: query", http.StatusBadRequest)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	selectStatement := "SELECT id, name, image_src FROM users WHERE name ILIKE FORMAT('%s%%', $1::text)"
	rows, err := db.Query(selectStatement, query)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users = []UserCondensed{}
	for rows.Next() {
		var user UserCondensed
		if err := rows.Scan(&user.ID, &user.Name, &user.ImageSource); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		users = append(users, user)

		if len(users) == 5 {
			break
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getActivity(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	ID := r.URL.Query().Get("id")
	requestingID := r.URL.Query().Get("requesting_id")
	if ID == "" || requestingID == "" {
		http.Error(w, "Missing query params: id and requesting_id", http.StatusBadRequest)
		return
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 0 {
		limit = 3
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	reviewQuery := "SELECT r.id, r.entity_id, r.type, r.colour, r.image_src, r.title, r.subtitle, r.score, r.body, r.created_on, u.id, u.name, u.image_src FROM reviews r JOIN users u ON u.id = r.user_id WHERE r.user_id = $1 ORDER BY r.created_on DESC;"

	reviewRows, err := db.Query(reviewQuery, ID)
	if err != nil {
		slog.Error("could not get timeline", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer reviewRows.Close()

	response := []TimelineResponse{}
	for reviewRows.Next() {
		var author UserCondensed
		var timelineElement TimelineResponse
		var reviewBag ReviewBag
		if err := reviewRows.Scan(&reviewBag.ID, &reviewBag.EntityID, &reviewBag.Type, &reviewBag.Colour, &reviewBag.ImageSource, &reviewBag.Title, &reviewBag.Subtitle, &reviewBag.Score, &reviewBag.Body, &timelineElement.Timestamp, &author.ID, &author.Name, &author.ImageSource); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		timelineElement.Author = author
		timelineElement.Data = reviewBag
		timelineElement.Type = ReviewType

		response = append(response, timelineElement)
	}

	listQuery := "SELECT l.id, l.type, l.colour, l.title, l.created_on, u.id, u.name, u.image_src FROM lists l JOIN users u ON u.id = l.user_id WHERE l.user_id = $1 ORDER BY l.created_on DESC;"

	listRows, err := db.Query(listQuery, ID)
	if err != nil {
		slog.Error("could not get timeline", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer listRows.Close()

	for listRows.Next() {
		var author UserCondensed
		var timelineElement TimelineResponse
		var listBag ListBag
		if err := listRows.Scan(&listBag.ID, &listBag.Type, &listBag.Colour, &listBag.Title, &timelineElement.Timestamp, &author.ID, &author.Name, &author.ImageSource); err != nil {
			slog.Error("could not get timeline", "error", err)
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		listElementQuery := "SELECT entity_id, title, image_src FROM list_elements WHERE list_id = $1 ORDER BY placement ASC;"

		listElementRows, err := db.Query(listElementQuery, listBag.ID)
		if err != nil {
			slog.Error("could not get timeline", "error", err)
			http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
			return
		}
		defer listElementRows.Close()

		var listElements []ListElement
		for listElementRows.Next() {
			var listElement ListElement
			if err := listElementRows.Scan(&listElement.EntityID, &listElement.Name, &listElement.ImageSrc); err != nil {
				slog.Error("could not get timeline", "error", err)
				http.Error(w, "Failed to scan row", http.StatusInternalServerError)
				return
			}

			listElements = append(listElements, listElement)
		}

		listBag.ListElements = listElements
		timelineElement.Author = author
		timelineElement.Data = listBag
		timelineElement.Type = ListType

		response = append(response, timelineElement)
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].Timestamp.After(response[j].Timestamp)
	})

	start := min(len(response), offset)
	end := min(len(response), offset+limit)

	response = response[start:end]

	for i, post := range response {
		var entityIdentifier string
		var tableName string
		var entityID any
		if post.Type == ReviewType {
			entityIdentifier = "review_id"
			tableName = "review_likes"

			reviewBag, ok := post.Data.(ReviewBag)
			if !ok {
				slog.Error("could not get timeline")
				http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
				return
			}

			entityID = reviewBag.ID
		} else if post.Type == ListType {
			entityIdentifier = "list_id"
			tableName = "list_likes"

			listBag, ok := post.Data.(ListBag)
			if !ok {
				slog.Error("could not get timeline")
				http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
				return
			}

			entityID = listBag.ID
		}

		likeCountQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", tableName, entityIdentifier)

		var numLikes int
		err = db.QueryRow(likeCountQuery, entityID).Scan(&numLikes)
		if err != nil {
			slog.Error("failed to execute SQL statement", "error", err)
			http.Error(w, "Failed to add user", http.StatusInternalServerError)
			return
		}

		isLikedQuery := likeCountQuery + " AND user_id = $2"

		var isLiked int
		err = db.QueryRow(isLikedQuery, entityID, requestingID).Scan(&isLiked)
		if err != nil {
			slog.Error("failed to execute SQL statement", "error", err)
			http.Error(w, "Failed to add user", http.StatusInternalServerError)
			return
		}

		response[i].NumLikes = numLikes
		response[i].IsLiked = isLiked > 0
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func followUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}

	token := r.Header["Authorization"][0][len("Bearer: "):]
	followerID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	var followUserBody followUserParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&followUserBody); err != nil {
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

	query := "INSERT INTO follower_relation (follower_id, followee_id) VALUES ($1, $2);"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(followerID, followUserBody.ID)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to follow user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}

func unfollowUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	token := r.Header["Authorization"][0][len("Bearer: "):]
	unfollowerID, err := extractUserIDFromJWTPayload(token)
	if err != nil {
		http.Error(w, "Malformed authentication token", http.StatusUnauthorized)
		return
	}

	var unfollowUserBody followUserParams
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&unfollowUserBody); err != nil {
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

	query := "DELETE FROM follower_relation WHERE follower_id = $1 AND followee_id = $2;"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(unfollowerID, unfollowUserBody.ID)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to follow user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}
