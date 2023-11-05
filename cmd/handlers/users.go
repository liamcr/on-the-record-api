package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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

type User struct {
	Provider    string      `json:"provider"`
	ProviderID  string      `json:"providerId"`
	Name        string      `json:"name"`
	ImageSource string      `json:"imageSrc"`
	Colour      string      `json:"colour"`
	MusicNotes  []MusicNote `json:"musicNotes"`
	CreatedOn   time.Time   `json:"createdOn"`
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
