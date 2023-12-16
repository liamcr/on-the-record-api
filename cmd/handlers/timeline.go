package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"time"
)

const (
	ReviewType int = iota
	ListType
)

type ListBag struct {
	ID           string        `json:"id"`
	Type         int           `json:"type"`
	Title        string        `json:"title"`
	Colour       string        `json:"colour"`
	ListElements []ListElement `json:"listElements"`
}

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
