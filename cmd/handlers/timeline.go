package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const (
	ReviewType int = iota
	ListType
)

type ReviewBag struct {
	ID          int    `json:"id"`
	Type        int    `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Colour      string `json:"colour"`
	ImageSource string `json:"imageSrc"`
	Score       int    `json:"score"`
	Body        string `json:"body"`
}

type Author struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"providerId"`
	Src        string `json:"imageSrc"`
	Name       string `json:"name"`
}

type TimelineResponse struct {
	Author    Author      `json:"author"`
	Type      int         `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func getTimeline(w http.ResponseWriter, r *http.Request) {
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

	rows, err := db.Query(reviewQuery, provider, providerID)
	if err != nil {
		slog.Error("could not get timeline", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	response := []TimelineResponse{}
	for rows.Next() {
		var author Author
		var timelineElement TimelineResponse
		var reviewBag ReviewBag
		if err := rows.Scan(&reviewBag.ID, &reviewBag.Type, &reviewBag.Colour, &reviewBag.ImageSource, &reviewBag.Title, &reviewBag.Subtitle, &reviewBag.Score, &reviewBag.Body, &timelineElement.Timestamp, &author.Provider, &author.ProviderID, &author.Name, &author.Src); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		timelineElement.Author = author
		timelineElement.Data = reviewBag
		timelineElement.Type = ReviewType

		response = append(response, timelineElement)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
