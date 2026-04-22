package main

import (
	"intelliqe/internal/api"
	"intelliqe/internal/service"
	mw "intelliqe/middleware"
	"intelliqe/store"
	"log"
	"net/http"
	"os"
)

func main() {
	log.Printf("Main is starting")
	var (
		PORT         string
		DATABASE_URL string
	)

	if PORT = os.Getenv("PORT"); PORT == "" {
		PORT = "8080"
	}

	if DATABASE_URL = os.Getenv("DATABASE_URL"); DATABASE_URL == "" {
		DATABASE_URL = "profile.db"
	}

	dbh := store.NewDBHandler(DATABASE_URL)
	svc := service.NewService(dbh)
	handler := api.NewHandler(svc)

	corsConfig := map[string]string{
		"Access-Control-Allow-Origin":  "*",
	}
	mux := handler.Routes()
	server := http.Server{
		Addr:    ":" + PORT,
		Handler: mw.CORS(mux, corsConfig),
	}

	log.Printf("Server is listening at: %v\n Database file: %v\n", server.Addr, DATABASE_URL)
	err := server.ListenAndServe()
	log.Fatal(err)
}
