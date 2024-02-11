package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/user", getUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/featured", getFeaturedUsers).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/search", searchUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/activity", getActivity).Methods("GET", "OPTIONS")
	r.HandleFunc("/user", addUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/user/follow", followUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/user/unfollow", unfollowUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/user", updateUser).Methods("PUT", "OPTIONS")
	r.HandleFunc("/user", deleteUser).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/review", addReview).Methods("POST", "OPTIONS")
	r.HandleFunc("/review", deleteReview).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/list", addList).Methods("POST", "OPTIONS")
	r.HandleFunc("/list", deleteList).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/timeline", getTimeline).Methods("GET", "OPTIONS")
	http.Handle("/", r)
}
