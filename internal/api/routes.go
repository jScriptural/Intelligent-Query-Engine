package api

import (
	mw "intelliqe/middleware"
	"net/http"
)

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /",
		h.HandleHealthCheck,
	)

	mux.Handle(
		"GET /api/profiles",
		mw.Auth(http.HandlerFunc(h.HandleQuery)),
	)
	mux.Handle(
		"GET /api/profiles/{id}",
		mw.Auth(http.HandlerFunc(h.HandleProfileRetrievalByID)),
	)
	mux.Handle(
		"GET /api/profiles/search",
		mw.Auth(http.HandlerFunc(h.HandleNLP)),
	)
	mux.Handle(
		"POST /api/profiles",
		mw.Auth(http.HandlerFunc(h.HandleProfileCreation)),
	)
	mux.Handle(
		"GET /api/profiles/export",
		mw.Auth(http.HandlerFunc(h.HandleDataExport)),
	)
	mux.Handle(
		"DELETE /api/profiles/{id}",
		mw.Auth(http.HandlerFunc(h.HandleProfileDeletionByID)),
	)

	mux.HandleFunc(
		"POST /auth/github",
		h.HandleGithubOAuth,
	)
	mux.HandleFunc(
		"POST /auth/logout",
		h.HandleLogout,
	)
	mux.HandleFunc(
		"POST /auth/refresh",
		h.HandleSessionRefresh,
	)

	return mux
}
