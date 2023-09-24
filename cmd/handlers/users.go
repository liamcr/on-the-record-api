package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

type addUserParams struct {
	Provider string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Name string `json:"name"`
	ImageSource string `json:"image_src"`
	Colour string `json:"colour"`
}

type User struct {
	Provider string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Name string `json:"name"`
	ImageSource string `json:"image_src"`
	Colour string `json:"colour"`
	CreatedOn time.Time `json:"created_on"`
}

func getUser(w http.ResponseWriter, r *http.Request) {
	db, err := connectToDB()
    if err != nil {
        slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
    }
    defer db.Close()

	selectStatement := "SELECT * FROM users WHERE provider = $1 AND provider_id = $2"

	provider := r.URL.Query().Get("provider")
	providerID := r.URL.Query().Get("provider_id")
	if provider == "" || providerID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
		return
	}
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
	if len(users) > 1{
		slog.Error("expected 1 user, found more", "matching_users", len(users), "provider", provider, "provider_id", providerID)
		http.Error(w, "found too many matching users", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users[0])
}

func addUser(w http.ResponseWriter, r *http.Request) {
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
	}();

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

	query := "INSERT INTO users (provider, provider_id, name, colour, image_src, created_on) VALUES ($1, $2, $3, $4, $5, $6);"

	stmt, err := db.Prepare(query)
    if err != nil {
		slog.Error("failed to prepare SQL statement", "error", err)
        http.Error(w, "Failed to prepare SQL statement", http.StatusInternalServerError)
        return
    }
    defer stmt.Close()

	createdOn := time.Now()
	_, err = stmt.Exec(addUserBody.Provider, addUserBody.ProviderID, addUserBody.Name, addUserBody.Colour, addUserBody.ImageSource, createdOn)
    if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
        http.Error(w, "Failed to add user", http.StatusInternalServerError)
        return
    }

	resp := User{
		Provider: addUserBody.Provider,
		ProviderID: addUserBody.ProviderID,
		Name: addUserBody.Name,
		ImageSource: addUserBody.ImageSource,
		Colour: addUserBody.Colour,
		CreatedOn: createdOn,
	}

	w.WriteHeader(http.StatusCreated)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
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
	}();

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

	query := "UPDATE users SET name = $3, colour = $4, image_src = $5 WHERE provider = $1 AND provider_id = $2;"

	_, err = db.Exec(query, updateUserBody.Provider, updateUserBody.ProviderID, updateUserBody.Name, updateUserBody.Colour, updateUserBody.ImageSource)
    if err != nil {
		slog.Error("failed to execute SQL statement", "error", err)
        http.Error(w, "Failed to add user", http.StatusInternalServerError)
        return
    }

	resp := addUserParams{
		Provider: updateUserBody.Provider,
		ProviderID: updateUserBody.ProviderID,
		Name: updateUserBody.Name,
		ImageSource: updateUserBody.ImageSource,
		Colour: updateUserBody.Colour,
	}

	w.WriteHeader(http.StatusOK)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	db, err := connectToDB()
    if err != nil {
        slog.Error("could not connect to Postgres", "error", err)
		http.Error(w, "Failed to connect to Postgres", http.StatusInternalServerError)
		return
    }
    defer db.Close()

	deleteSQL := "DELETE FROM users WHERE provider = $1 AND provider_id = $2"

	provider := r.URL.Query().Get("provider")
	providerID := r.URL.Query().Get("provider_id")
	if provider == "" || providerID == "" {
		http.Error(w, "Missing query params: provider and provider_id", http.StatusBadRequest)
		return
	}
	result, err := db.Exec(deleteSQL, provider, providerID)
	if err != nil {
		slog.Error("could not delete user", "error", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil || rowsDeleted == 0 {
		slog.Error("no user with specified ID", "error", err, "provider", provider, "provider_id", providerID)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
}