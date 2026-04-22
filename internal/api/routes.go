package api

import (
	"net/http"
)

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", h.HandleHealthCheck)
	mux.HandleFunc("GET /api/profiles", h.HandleQuery)
	mux.HandleFunc("GET /api/profiles/search", h.HandleNLP)

	return mux
}
