package handlers

import (
	"net/http"
	"on-the-record-api/cmd/middleware"

	"github.com/gorilla/mux"
)

func RegisterHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/user", getUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/featured", getFeaturedUsers).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/search", searchUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/activity", getActivity).Methods("GET", "OPTIONS")
	r.HandleFunc(
		"/user",
		middleware.EnsureValidToken()(http.HandlerFunc(addUser)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/user/follow",
		middleware.EnsureValidToken()(http.HandlerFunc(followUser)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/user/unfollow",
		middleware.EnsureValidToken()(http.HandlerFunc(unfollowUser)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/user",
		middleware.EnsureValidToken()(http.HandlerFunc(updateUser)).ServeHTTP,
	).Methods("PUT", "OPTIONS")
	r.HandleFunc(
		"/user",
		middleware.EnsureValidToken()(http.HandlerFunc(deleteUser)).ServeHTTP,
	).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/review/likes", getReviewLikes).Methods("GET", "OPTIONS")
	r.HandleFunc("/review", addReview).Methods("POST", "OPTIONS")
	r.HandleFunc("/review/like", likeReview).Methods("POST", "OPTIONS")
	r.HandleFunc("/review/unlike", unlikeReview).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/review",
		middleware.EnsureValidToken()(http.HandlerFunc(deleteReview)).ServeHTTP,
	).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/list/likes", getListLikes).Methods("GET", "OPTIONS")
	r.HandleFunc("/list", addList).Methods("POST", "OPTIONS")
	r.HandleFunc("/list/like", likeList).Methods("POST", "OPTIONS")
	r.HandleFunc("/list/unlike", unlikeList).Methods("POST", "OPTIONS")
	r.HandleFunc("/list", deleteList).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/timeline", getTimeline).Methods("GET", "OPTIONS")
	http.Handle("/", r)
}
