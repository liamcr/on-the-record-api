package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

type addUserParams struct {
	Provider    string      `json:"provider"`
	ProviderID  string      `json:"providerId"`
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	MusicNotes  []MusicNote `json:"musicNotes"`
}

type followUserParams struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"providerId"`
}

type User struct {
	Provider    string      `json:"provider"`
	ProviderID  string      `json:"providerId"`
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	Followers   int         `json:"followers"`
	Following   int         `json:"following"`
	IsFollowing bool        `json:"isFollowing"`
	MusicNotes  []MusicNote `json:"musicNotes"`
	CreatedOn   time.Time   `json:"createdOn"`
}

type UserCondensed struct {
	Provider    string `json:"provider"`
	ProviderID  string `json:"providerId"`
	Name        string `json:"name"`
	ImageSource string `json:"imageSrc"`
}

type MusicNote struct {
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
	provider := r.URL.Query().Get("provider")
	providerID := r.URL.Query().Get("provider_id")
	requestingProvider := r.URL.Query().Get("requesting_provider")
	requestingProviderID := r.URL.Query().Get("requesting_provider_id")
	if provider == "" || providerID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	selectStatement := "SELECT provider, provider_id, name, colour, image_src, created_on FROM users WHERE provider = $1 AND provider_id = $2"
	rows, err := db.Query(selectStatement, provider, providerID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Provider, &user.ProviderID, &user.Name, &user.Colour, &user.ImageSource, &user.CreatedOn); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		slog.Error("could not find user", "provider", provider, "provider_id", providerID)
		http.Error(w, "Couldn't find user", http.StatusNotFound)
		return
	}
	if len(users) > 1 {
		slog.Error("expected 1 user, found more", "matching_users", len(users), "provider", provider, "provider_id", providerID)
		http.Error(w, "found too many matching users", http.StatusInternalServerError)
		return
	}

	numFollowersStatement := "SELECT count(*) FROM follower_relation WHERE followee_provider = $1 AND followee_provider_id = $2"
	rows, err = db.Query(numFollowersStatement, provider, providerID)
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

	numFollowingStatement := "SELECT count(*) FROM follower_relation WHERE follower_provider = $1 AND follower_provider_id = $2"
	rows, err = db.Query(numFollowingStatement, provider, providerID)
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
	if requestingProvider != "" && requestingProviderID != "" {
		isCurrentUserFollowingQuery := "SELECT count(*) FROM follower_relation WHERE follower_provider = $1 AND follower_provider_id = $2 AND followee_provider= $3 AND followee_provider_id = $4"
		rows, err = db.Query(isCurrentUserFollowingQuery, requestingProvider, requestingProviderID, provider, providerID)
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

	query := "SELECT prompt, image_src, title, subtitle FROM music_notes WHERE user_provider = $1 AND user_provider_id = $2;"
	rows, err = db.Query(query, provider, providerID)
	if err != nil {
		slog.Error("could not get user", "error", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	var musicNotes []MusicNote
	for rows.Next() {
		var musicNote MusicNote
		if err := rows.Scan(&musicNote.Prompt, &musicNote.ImageSource, &musicNote.Title, &musicNote.Subtitle); err != nil {
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

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user doesn't already have an account
	checkIfUserExistsQuery := "SELECT COUNT(*) FROM users WHERE provider = $1 AND provider_id = $2"

	var count int
	err = db.QueryRow(checkIfUserExistsQuery, addUserBody.Provider, addUserBody.ProviderID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		slog.Error("this user already has an account", "provider", addUserBody.Provider, "provider_id", addUserBody.ProviderID)
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

	query := "INSERT INTO users (provider, provider_id, name, colour, image_src, created_on) VALUES ($1, $2, $3, $4, $5, $6);"

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	createdOn := time.Now()
	_, err = stmt.Exec(addUserBody.Provider, addUserBody.ProviderID, addUserBody.Name, addUserBody.Colour, addUserBody.ImageSource, createdOn)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	insertMusicNoteQuery := "INSERT INTO music_notes (user_provider, user_provider_id, prompt, image_src, title, subtitle) VALUES ($1, $2, $3, $4, $5, $6);"
	for _, musicNote := range addUserBody.MusicNotes {
		stmt, err := tx.Prepare(insertMusicNoteQuery)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to prepare SQL statement", "error", err)
			http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(addUserBody.Provider, addUserBody.ProviderID, musicNote.Prompt, musicNote.ImageSource, musicNote.Title, musicNote.Subtitle)
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
		Provider:    addUserBody.Provider,
		ProviderID:  addUserBody.ProviderID,
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
	var updateUserBody addUserParams
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

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Make sure user user exists
	checkIfUserExistsQuery := "SELECT COUNT(*) FROM users WHERE provider = $1 AND provider_id = $2"

	var count int
	err = db.QueryRow(checkIfUserExistsQuery, updateUserBody.Provider, updateUserBody.ProviderID).Scan(&count)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		slog.Error("could not find user", "provider", updateUserBody.Provider, "provider_id", updateUserBody.ProviderID)
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

	query := "UPDATE users SET name = $3, colour = $4, image_src = $5 WHERE provider = $1 AND provider_id = $2;"

	_, err = tx.Exec(query, updateUserBody.Provider, updateUserBody.ProviderID, updateUserBody.Name, updateUserBody.Colour, updateUserBody.ImageSource)
	if err != nil {
		tx.Rollback()
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to add user", http.StatusInternalServerError)
		return
	}

	// Delete existing music notes in order to overwrite them with the new ones
	query = "DELETE FROM music_notes WHERE user_provider = $1 AND user_provider_id = $2"

	_, err = tx.Exec(query, updateUserBody.Provider, updateUserBody.ProviderID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	insertMusicNoteQuery := "INSERT INTO music_notes (user_provider, user_provider_id, prompt, image_src, title, subtitle) VALUES ($1, $2, $3, $4, $5, $6);"
	for _, musicNote := range updateUserBody.MusicNotes {
		stmt, err := tx.Prepare(insertMusicNoteQuery)
		if err != nil {
			tx.Rollback()
			slog.Error("failed to prepare SQL statement", "error", err)
			http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(updateUserBody.Provider, updateUserBody.ProviderID, musicNote.Prompt, musicNote.ImageSource, musicNote.Title, musicNote.Subtitle)
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

	resp := addUserParams{
		Provider:    updateUserBody.Provider,
		ProviderID:  updateUserBody.ProviderID,
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

	provider := r.URL.Query().Get("provider")
	providerID := r.URL.Query().Get("provider_id")
	if provider == "" || providerID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("could not begin transaction", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}

	query := "DELETE FROM music_notes WHERE user_provider = $1 AND user_provider_id = $2"

	_, err = tx.Exec(query, provider, providerID)
	if err != nil {
		tx.Rollback()
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	query = "DELETE FROM users WHERE provider = $1 AND provider_id = $2"

	_, err = tx.Exec(query, provider, providerID)
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

	selectStatement := "SELECT provider, provider_id, name, image_src FROM users WHERE name ILIKE FORMAT('%s%%', $1::text)"
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
		if err := rows.Scan(&user.Provider, &user.ProviderID, &user.Name, &user.ImageSource); err != nil {
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
	provider := r.URL.Query().Get("provider")
	providerID := r.URL.Query().Get("provider_id")
	if provider == "" || providerID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
		return
	}

	db, err := connectToDB()
	if err != nil {
		slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	reviewQuery := "SELECT r.id, r.type, r.colour, r.image_src, r.title, r.subtitle, r.score, r.body, r.created_on, u.provider, u.provider_id, u.name, u.image_src FROM reviews r JOIN users u ON u.provider = r.user_provider AND u.provider_id = r.user_provider_id WHERE r.user_provider = $1 AND r.user_provider_id = $2 ORDER BY r.created_on DESC;"

	reviewRows, err := db.Query(reviewQuery, provider, providerID)
	if err != nil {
		slog.Error("could not get timeline", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer reviewRows.Close()

	response := []TimelineResponse{}
	for reviewRows.Next() {
		var author Author
		var timelineElement TimelineResponse
		var reviewBag ReviewBag
		if err := reviewRows.Scan(&reviewBag.ID, &reviewBag.Type, &reviewBag.Colour, &reviewBag.ImageSource, &reviewBag.Title, &reviewBag.Subtitle, &reviewBag.Score, &reviewBag.Body, &timelineElement.Timestamp, &author.Provider, &author.ProviderID, &author.Name, &author.Src); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		timelineElement.Author = author
		timelineElement.Data = reviewBag
		timelineElement.Type = ReviewType

		response = append(response, timelineElement)
	}

	listQuery := "SELECT l.id, l.type, l.colour, l.title, l.created_on, u.provider, u.provider_id, u.name, u.image_src FROM lists l JOIN users u ON u.provider = l.user_provider AND u.provider_id = l.user_provider_id WHERE l.user_provider = $1 AND l.user_provider_id = $2 ORDER BY l.created_on DESC;"

	listRows, err := db.Query(listQuery, provider, providerID)
	if err != nil {
		slog.Error("could not get timeline", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer listRows.Close()

	for listRows.Next() {
		var author Author
		var timelineElement TimelineResponse
		var listBag ListBag
		if err := listRows.Scan(&listBag.ID, &listBag.Type, &listBag.Colour, &listBag.Title, &timelineElement.Timestamp, &author.Provider, &author.ProviderID, &author.Name, &author.Src); err != nil {
			slog.Error("could not get timeline", "error", err)
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		listElementQuery := "SELECT title, image_src FROM list_elements WHERE list_id = $1 ORDER BY placement ASC;"

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
			if err := listElementRows.Scan(&listElement.Name, &listElement.ImageSrc); err != nil {
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

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func followUser(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	followerProvider := r.URL.Query().Get("provider")
	followerProviderID := r.URL.Query().Get("provider_id")
	if followerProvider == "" || followerProviderID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
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

	query := "INSERT INTO follower_relation (follower_provider, follower_provider_id, followee_provider, followee_provider_id) VALUES ($1, $2, $3, $4);"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(followerProvider, followerProviderID, followUserBody.Provider, followUserBody.ProviderID)
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
	unfollowerProvider := r.URL.Query().Get("provider")
	unfollowerProviderID := r.URL.Query().Get("provider_id")
	if unfollowerProvider == "" || unfollowerProviderID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
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

	query := "DELETE FROM follower_relation WHERE follower_provider = $1 AND follower_provider_id = $2 AND followee_provider = $3 AND followee_provider_id = $4;"

	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
		http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(unfollowerProvider, unfollowerProviderID, unfollowUserBody.Provider, unfollowUserBody.ProviderID)
	if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
		http.Error(w, "Failed to follow user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}
