package api

import (
	mw "intelliqe/middleware"
	"net/http"
	"time"
)

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	apiLimiter := mw.NewRateLimiter(60, 60*time.Second)
	authLimiter := mw.NewRateLimiter(10, 60*time.Second)

	mux.Handle(
		"GET /",
		apiLimiter(
			http.HandlerFunc(
				h.HandleHealthCheck,
			),
		),
	)

	mux.Handle(
		"GET /api/profiles",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleQuery,
				),
			),
		),
	)
	mux.Handle(
		"GET /api/profiles/{id}",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleProfileRetrievalByID,
				),
			),
		),
	)
	mux.Handle(
		"GET /api/profiles/search",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleNLP,
				),
			),
		),
	)
	mux.Handle(
		"POST /api/profiles",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleProfileCreation,
				),
			),
		),
	)
	mux.Handle(
		"GET /api/profiles/export",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleDataExport,
				),
			),
		),
	)
	mux.Handle(
		"DELETE /api/profiles/{id}",
		mw.Auth(
			apiLimiter(
				http.HandlerFunc(
					h.HandleProfileDeletionByID,
				),
			),
		),
	)

	mux.Handle(
		"POST /auth/github",
		authLimiter(
			http.HandlerFunc(
				h.HandleGithubOAuth,
			),
		),
	)
	mux.Handle(
		"POST /auth/logout",
		authLimiter(
			http.HandlerFunc(
				h.HandleLogout,
			),
		),
	)
	mux.Handle(
		"POST /auth/refresh",
		authLimiter(
			http.HandlerFunc(
				h.HandleSessionRefresh,
			),
		),
	)

	return mux
}
