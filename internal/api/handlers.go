package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"intelliqe/internal/models"
	"intelliqe/internal/service"
	"io"
	"log"
	"net/http"
	"net/url"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(s *service.Service) *Handler {
	return &Handler{svc: s}
}

func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	log.Printf("Ctx: %#v", r.Context())

	query := r.URL.Query()
	log.Printf("query: %v", query)

	p, total, err := h.svc.Filter(r.Context(), query)

	if err != nil {
		log.Printf("Error: %s\n", err)
		h.errorMux(w, err)
		return
	}

	h.sendResponse(
		w,
		http.StatusOK,
		"success",
		query,
		total,
		p,
	)

}

func (h *Handler) HandleNLP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	nl := query.Get("q")
	log.Printf("q: %v", nl)
	log.Printf("query: %v", query)

	if nl != "" {
		delete(query, "q")
	}

	p, total, err := h.svc.NLProcessor(r.Context(), query, nl)
	if err != nil {
		log.Printf("HandleNLP: %v", err)
		h.errorMux(w, err)
		return
	}

	h.sendResponse(
		w,
		http.StatusOK,
		"success",
		query,
		total,
		p,
	)

}

func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendError(
		w,
		http.StatusOK,
		"success",
		"Server is up",
	)
}

func (h *Handler) HandleProfileCreation(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value("role")
	if role != "admin" {
		log.Println("role: ", role)
		h.errorMux(w, models.ErrForbidden)
		return
	}
	defer r.Body.Close()
	var data models.PostData
	if err := h.decode(r.Body, &data); err != nil {
		log.Printf("%v", err)
		h.errorMux(
			w,
			fmt.Errorf("HandleProfileCreation: %w", models.Err502),
		)

		return
	}

	log.Printf("postData: %v", data)
	if strings.TrimSpace(data.Name) == "" {
		h.errorMux(
			w,
			fmt.Errorf("HandleProfileCreation: %w", models.ErrEmptyParam),
		)
		return
	}
	data.Name = strings.ToLower(strings.TrimSpace(data.Name))

	p, isNew, err := h.svc.GetOrCreateProfile(r.Context(), data.Name)

	if err != nil {
		log.Printf("%v", err)
		h.errorMux(w, err)
		return
	}

	q := url.Values{}
	q.Set("limit", "1")
	q.Set("page", "1")
	total := 1
	if isNew {
		h.sendResponse(
			w,
			http.StatusCreated,
			"success",
			q,
			total,
			[]*models.Profile{p},
		)
		return
	}

	h.sendResponse(
		w,
		http.StatusOK,
		"success",
		q,
		total,
		[]*models.Profile{p},
	)
	return
}

func (h *Handler) HandleProfileRetrievalByID(w http.ResponseWriter, r *http.Request) {
	id := h.removeAllWhitespaces(r.PathValue("id"))
	if id == "" {
		h.errorMux(
			w,
			fmt.Errorf("HandleProfileRetrievalByID: %w", models.ErrEmptyParam),
		)
		return
	}

	log.Printf("%v", id)
	p, err := h.svc.RetrieveProfileByID(r.Context(), id)
	if err != nil {
		log.Printf("%v", err)
		h.errorMux(w, err)
		return
	}

	q := url.Values{}
	q.Set("limit", "1")
	q.Set("page", "1")
	total := 1
	h.sendResponse(
		w,
		http.StatusOK,
		"success",
		q,
		total,
		[]*models.Profile{p},
	)
}

func (h *Handler) HandleProfileDeletionByID(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value("role")
	if role != "admin" {
		log.Println("role: ", role)
		h.errorMux(w, models.ErrForbidden)
		return
	}
	id := r.PathValue("id")
	id = h.removeAllWhitespaces(id)

	if id == "" {
		h.errorMux(
			w,
			fmt.Errorf("HandleProfileDeletion: %w", models.ErrEmptyParam),
		)
		return
	}
	_, err := uuid.Parse(id)
	if err != nil {
		log.Printf("%v", err)
		h.errorMux(
			w,
			fmt.Errorf("HandleProfileDeletion: %w", models.ErrInvalidParam),
		)
		return
	}

	err = h.svc.DeleteProfile(r.Context(), id)
	if err != nil {
		log.Printf("HandleProfileDeletion: %s", err)
		h.errorMux(w, err)
	}

	log.Printf("Deleted: %v", id)
	q := url.Values{}
	h.sendResponse(
		w,
		http.StatusNoContent,
		"success",
		q,
		0,
		nil,
	)

}

func (h *Handler) HandleGithubOAuth(w http.ResponseWriter, r *http.Request) {
	tmpCode := models.GitTmpCode{}
	err := h.decode(r.Body, &tmpCode)
	if err != nil {
		log.Println(err)
		h.errorMux(
			w,
			fmt.Errorf("HandleGithubOAuth: %w", models.ErrEmptyBody),
		)
		return
	}

	token, err := h.svc.GetAuthCredential(r.Context(), tmpCode)
	if err != nil {
		log.Println(err)
		h.errorMux(w, err)
		return
	}

	h.sendToken(w, http.StatusOK, token)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	userID := models.UserID{}
	err := h.decode(r.Body, &userID)
	if err != nil {
		log.Printf("%#v\n", err)
		h.errorMux(
			w,
			err,
		)
		return
	}

	err = h.svc.RevokeToken(r.Context(), &userID)
	if err != nil {
		log.Printf("%#v\n", err)
		h.errorMux(w, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := models.RefreshToken{}
	err := h.decode(r.Body, &refreshToken)
	if err != nil {
		log.Println(err)
		h.errorMux(
			w,
			models.ErrEmptyBody,
		)
		return
	}

	claims := models.RefreshClaims{}

	user, err := h.svc.ValidateToken(r.Context(), refreshToken.Token, &claims)
	if err != nil {
		log.Println(err)
		h.errorMux(w, err)
		return
	}

	cred, err := h.svc.GenerateCredential(user.ID, user.Username, user.Role)
	if err != nil {
		log.Printf("%v", err)
		h.errorMux(w, err)
		return
	}

	err = h.svc.UpdateUserToken(r.Context(),user.ID.String(),cred.RefreshToken)
	if err != nil {
		log.Println(err)
		h.errorMux(w, err)
		return
	}

	h.sendToken(w, http.StatusOK, cred)
}

func (h *Handler) HandleFileExport(w htpp.ResponseWriter, r *http.Request) {
	
}
/***************************************
*                                      *
*            HELPER FUNCS              *
*                                      *
****************************************/

func (h *Handler) errorMux(w http.ResponseWriter, err error) {

	switch {
	case errors.Is(err, models.ErrEmptyParam):
		h.sendError(
			w,
			http.StatusBadRequest,
			"error",
			"Missing or empty parameter",
		)
	case errors.Is(err, models.ErrInvalidParam):
		h.sendError(
			w,
			http.StatusUnprocessableEntity,
			"error",
			"Invalid parameter type",
		)
	case errors.Is(err, models.Err502):
		h.sendError(
			w,
			http.StatusBadGateway,
			"error",
			"Upstream error",
		)
	case errors.Is(err, models.ErrNotFound):
		h.sendError(
			w,
			http.StatusNotFound,
			"error",
			"Profile not found",
		)
	case errors.Is(err, models.ErrUnInterpretable):
		h.sendError(
			w,
			http.StatusBadRequest,
			"error",
			"Unable to interprete query",
		)
	case errors.Is(err, models.ErrEmptyBody):
		h.sendError(
			w,
			http.StatusBadRequest,
			"error",
			"Empty Request body",
		)
	case errors.Is(err, models.ErrDeadlineExceeded):
		h.sendError(
			w,
			http.StatusBadRequest,
			"error",
			"Request timeout",
		)
	case errors.Is(err, models.ErrUnauthorized):
		h.sendError(
			w,
			http.StatusUnauthorized,
			"error",
			"Operation not authorized",
		)
	case errors.Is(err, models.ErrExpiredToken):
		h.sendError(
			w,
			http.StatusUnauthorized,
			"error",
			"Session expired",
		)
	case errors.Is(err, models.ErrForbidden):
		h.sendError(
			w,
			http.StatusForbidden,
			"error",
			"Permission denied",
		)
	case errors.Is(err, models.ErrNoSession):
		h.sendError(
			w,
			http.StatusUnauthorized,
			"error",
			"No session",
		)
	case errors.Is(err, models.ErrNoUser):
		h.sendError(
			w,
			http.StatusUnauthorized,
			"error",
			"No user",
		)
	default:
		h.sendError(
			w,
			http.StatusInternalServerError,
			"error",
			"Server failure",
		)
	}
}

func (h *Handler) sendError(w http.ResponseWriter, code int, status, msg string) {
	err := models.ErrResponse{
		Status:  status,
		Message: msg,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		log.Printf("Unable to write response: %v", err)
	}
}

func (h *Handler) sendResponse(w http.ResponseWriter, code int, status string, q url.Values, total int, data []*models.Profile) {

	limit, err1 := strconv.Atoi(q.Get("limit"))
	page, err2 := strconv.Atoi(q.Get("page"))

	if err1 != nil || err2 != nil {
		log.Printf("sendResponse: %v:%v", err1, err2)
		h.errorMux(
			w,
			models.ErrInvalidParam,
		)
		return
	}

	links := models.PageLinks{}
	res := models.Response{
		Status:     status,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(limit))),
		Links:      links,
		Data:       data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("Unable to write response: %v", err)
	}
}

func (h *Handler) decode(r io.Reader, v any) error {
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(v); err != nil {
		return err
	}
	return nil
}

func (h *Handler) removeAllWhitespaces(s string) string {
	trim := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return trim
}

func (h *Handler) sendToken(w http.ResponseWriter, statusCode int, token *models.AuthCredential) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(token); err != nil {
		log.Printf("Unable to write response: %v", err)
	}
}
