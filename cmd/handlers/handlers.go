package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/user", getUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user/search", searchUser).Methods("GET", "OPTIONS")
	r.HandleFunc("/user", addUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/user", updateUser).Methods("PUT", "OPTIONS")
	r.HandleFunc("/user", deleteUser).Methods("DELETE", "OPTIONS")
	http.Handle("/", r)
}
