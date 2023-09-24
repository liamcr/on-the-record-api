package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/user", getUser).Methods("GET")
	r.HandleFunc("/user", addUser).Methods("POST")
	r.HandleFunc("/user", updateUser).Methods("PUT")
	r.HandleFunc("/user", deleteUser).Methods("DELETE")
	http.Handle("/", r)
}
