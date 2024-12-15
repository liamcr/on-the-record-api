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
	r.HandleFunc(
		"/review",
		middleware.EnsureValidToken()(http.HandlerFunc(addReview)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/review/like",
		middleware.EnsureValidToken()(http.HandlerFunc(likeReview)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/review/unlike",
		middleware.EnsureValidToken()(http.HandlerFunc(unlikeReview)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/review",
		middleware.EnsureValidToken()(http.HandlerFunc(deleteReview)).ServeHTTP,
	).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/list/likes", getListLikes).Methods("GET", "OPTIONS")
	r.HandleFunc(
		"/list",
		middleware.EnsureValidToken()(http.HandlerFunc(addList)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/list/like",
		middleware.EnsureValidToken()(http.HandlerFunc(likeList)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/list/unlike",
		middleware.EnsureValidToken()(http.HandlerFunc(unlikeList)).ServeHTTP,
	).Methods("POST", "OPTIONS")
	r.HandleFunc(
		"/list",
		middleware.EnsureValidToken()(http.HandlerFunc(deleteList)).ServeHTTP,
	).Methods("DELETE", "OPTIONS")

	r.HandleFunc("/timeline", getTimeline).Methods("GET", "OPTIONS")
	http.Handle("/", r)
}
