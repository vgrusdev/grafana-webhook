package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type App struct {
	Router *mux.Router
}

func (a *App) Initialize() {
	a.Router = mux.NewRouter()
	// a.Router.HandleFunc("health", HealthCheck).Methods("GET")
	a.Router.HandleFunc("/alert", Alert).Methods("POST")
}

func (a *App) Run(addr string) {
	slog.Info("Running", "port", addr)
	srv := &http.Server{
		Handler:      a.Router,
		Addr:         ":" + addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func Alert(w http.ResponseWriter, r *http.Request) {
	var m map[string]interface{}
	json.NewDecoder(r.Body).Decode(&m)

	slog.Info("Alert", "json", m)

	respondWithJSON(w, http.StatusCreated, map[string]string{"result": "success"})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
