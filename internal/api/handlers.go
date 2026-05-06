package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"intelliqe/internal/models"
	"intelliqe/internal/service"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(s *service.Service) *Handler {
	return &Handler{svc: s}
}

func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	start := time.Now();
	defer h.logResponseTime(start);

	query := r.URL.Query()

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
	start := time.Now();
	defer h.logResponseTime(start);

	query := r.URL.Query()
	nl := query.Get("q")

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
	start := time.Now();
	defer h.logResponseTime(start);

	h.sendError(
		w,
		http.StatusOK,
		"success",
		"Server is up",
	)
}

func (h *Handler) HandleProfileCreation(w http.ResponseWriter, r *http.Request) {
	start := time.Now();
	defer h.logResponseTime(start);

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
			models.Err502,
		)

		return
	}

	log.Printf("postData: %v", data)
	if strings.TrimSpace(data.Name) == "" {
		h.errorMux(
			w,
			models.ErrEmptyParam,
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
	start := time.Now();
	defer h.logResponseTime(start);

	id := h.removeAllWhitespaces(r.PathValue("id"))
	if id == "" {
		h.errorMux(
			w,
			models.ErrEmptyParam,
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
	start := time.Now();
	defer h.logResponseTime(start);

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
			models.ErrEmptyParam,
		)
		return
	}
	_, err := uuid.Parse(id)
	if err != nil {
		log.Printf("%v", err)
		h.errorMux(
			w,
			models.ErrInvalidParam,
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
	start := time.Now();
	defer h.logResponseTime(start);

	tmpCode := models.GitTmpCode{}
	err := h.decode(r.Body, &tmpCode)
	if err != nil {
		log.Println(err)
		h.errorMux(
			w,
			models.ErrEmptyBody,
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
	start := time.Now();
	defer h.logResponseTime(start);

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
	start := time.Now();
	defer h.logResponseTime(start);

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

	err = h.svc.UpdateUserToken(r.Context(), user.ID.String(), cred.RefreshToken)
	if err != nil {
		log.Println(err)
		h.errorMux(w, err)
		return
	}

	h.sendToken(w, http.StatusOK, cred)
}

func (h *Handler) HandleDataExport(w http.ResponseWriter, r *http.Request) {
	start := time.Now();
	defer h.logResponseTime(start);

	supportedFormat := []string{"csv", "json"}

	query := r.URL.Query()
	format := query.Get("format")
	log.Println("format:", format)

	if format == "" {
		h.errorMux(
			w,
			models.ErrEmptyParam,
		)
		return
	}
	format = strings.ToLower(format)
	if !slices.Contains(supportedFormat, format) {
		h.errorMux(
			w,
			models.ErrInvalidParam,
		)
		return
	}
	query.Del("format")
	p, total, err := h.svc.Filter(r.Context(), query)
	if err != nil {
		log.Printf("HandleDataExport: %v", err)
		h.errorMux(w, err)
		return
	}
	if total == 0 {
		h.errorMux(w, models.ErrNotFound)
		return
	}

	switch format {
	case "csv":
		err := h.exportCSV(w, p)
		if err != nil {
			log.Printf("HandleDataExport: %v", err)
			h.errorMux(w, err)
			return
		}
	case "json":
		err := h.exportJSON(w, p)
		if err != nil {
			log.Printf("HandleDataExport: %v", err)
			h.errorMux(w, err)
			return
		}
	}
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

func (h *Handler) exportCSV(w http.ResponseWriter, p []*models.Profile) error {
	cw := csv.NewWriter(w)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("X-Content-Type-Option", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%q", "profiles_"+strconv.FormatInt(time.Now().Unix(), 10)+".csv"))

	w.WriteHeader(http.StatusOK)

	for _, v := range p {
		record := []string{
			v.ID.String(),
			v.Name,
			v.Gender,
			strconv.FormatFloat(v.GenderProbability, 'f', -1, 64),
			strconv.Itoa(v.Age),
			v.AgeGroup,
			v.CountryID,
			v.CountryName,
			strconv.FormatFloat(v.CountryProbability, 'f', -1, 64),
			v.CreatedAt.Format(time.RFC3339),
		}
		err := cw.Write(record)
		if err != nil {
			return fmt.Errorf("exportCSV: %w", err)
		}

	}
	cw.Flush()

	if cw.Error() != nil {
		return fmt.Errorf("exportCSV: %w", cw.Error())
	}

	return nil
}

func (h *Handler) exportJSON(w http.ResponseWriter, p []*models.Profile) error {

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Option", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%q", "profiles_"+strconv.FormatInt(time.Now().Unix(), 10)+".json"))

	w.WriteHeader(http.StatusOK)

	err := encoder.Encode(p)
	if err != nil {
		return fmt.Errorf("exportJSON: %w", err)
	}

	return nil
}

func (h *Handler)logResponseTime (start time.Time) {
	log.Printf("Response time: %v",time.Since(start).String());
}
