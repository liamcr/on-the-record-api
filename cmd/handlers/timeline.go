package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
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
	EntityID    string `json:"entityId"`
	Type        int    `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Colour      string `json:"colour"`
	ImageSource string `json:"imageSrc"`
	Score       int    `json:"score"`
	Body        string `json:"body"`
}

type TimelineResponse struct {
	Author    UserCondensed `json:"author"`
	Type      int           `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Data      interface{}   `json:"data"`
}

func getTimeline(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	if r.Method == "OPTIONS" {
		return
	}
	ID := r.URL.Query().Get("id")
	if ID == "" {
		http.Error(w, "Missing query param: id", http.StatusBadRequest)
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

	getFollowedUserIDsQuery := "SELECT followee_id FROM follower_relation WHERE follower_id = $1;"
	followedUserRows, err := db.Query(getFollowedUserIDsQuery, ID)
	if err != nil {
		slog.Error("could not get followed users", "error", err)
		http.Error(w, "Failed to get timeline", http.StatusInternalServerError)
		return
	}
	defer followedUserRows.Close()

	followedUsers := []string{}
	for followedUserRows.Next() {
		var userID string
		if err := followedUserRows.Scan(&userID); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}

		followedUsers = append(followedUsers, userID)
	}

	whereClause := "user_id = $1"
	for _, followedUser := range followedUsers {
		whereClause = fmt.Sprintf("%s OR user_id = '%s'", whereClause, followedUser)
	}

	reviewQuery := fmt.Sprintf("SELECT r.id, r.entity_id, r.type, r.colour, r.image_src, r.title, r.subtitle, r.score, r.body, r.created_on, u.id, u.name, u.image_src FROM reviews r JOIN users u ON u.id = r.user_id WHERE %s ORDER BY r.created_on DESC;", whereClause)

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

	listQuery := fmt.Sprintf("SELECT l.id, l.type, l.colour, l.title, l.created_on, u.id, u.name, u.image_src FROM lists l JOIN users u ON u.id = l.user_id WHERE %s ORDER BY l.created_on DESC;", whereClause)

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

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response[start:end])
}
