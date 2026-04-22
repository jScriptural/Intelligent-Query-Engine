package api

import (
	"errors"
	"intelliqe/internal/models"
	"intelliqe/internal/service"
	"log"
	"net/http"
	"net/url"
	"encoding/json"
	"strconv"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(s *service.Service) *Handler {
	return &Handler{svc: s}
}

func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	p,total,err := h.svc.Filter(r.Context(), query)

	if err != nil {
		log.Printf("Error: %s\n",err)
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
	log.Println("NLP")
	w.Write([]byte("NLP"))
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
	case errors.Is(err, models.ErrNotFound):
		h.sendError(
			w,
			http.StatusNotFound,
			"error",
			"Profile not found",
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

func (h *Handler)sendResponse(w http.ResponseWriter, code int, status string, q url.Values, total int, data []*models.Profile) {
	
	limit,err1 := strconv.Atoi(q.Get("limit"));
	page,err2 := strconv.Atoi(q.Get("page"));

	if err1 != nil || err2 != nil {
		h.errorMux(
			w,
			models.ErrInvalidParam,
		)
		return;
	}

	res := models.Response{
		Status: status,
		Page:   page,
		Limit:  limit,
		Total:  total,
		Data:   data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("Unable to write response: %v", err)
	}
}
