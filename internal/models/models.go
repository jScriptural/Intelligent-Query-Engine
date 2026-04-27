package models

import (
	"errors"
	"github.com/google/uuid"
	"time"
)

var (
	ErrNotFound        = errors.New("Not found")
	ErrInvalidParam    = errors.New("Unprocessable entity")
	ErrUnInterpretable = errors.New("Unable to interpret query")
	ErrEmptyParam      = errors.New("Bad request")
	Err502             = errors.New("Bad gateway or upstream error")
)

type Profile struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Gender             string    `json:"gender"`
	GenderProbability  float64   `json:"gender_probability"`
	Age                int       `json:"age"`
	AgeGroup           string    `json:"age_group"`
	CountryID          string    `json:"country_id"`
	CountryName        string    `json:"country_name,omitempty"`
	CountryProbability float64   `json:"country_probability"`
	CreatedAt          time.Time `json:"created_at"`
}

type PostData struct {
	Name string `json"name"`
}

type Response struct {
	Status string     `json:"status"`
	Page   int        `json:"page,omitempy"`
	Limit  int        `json:"limit,omitempty"`
	Total  int        `json:"total,omitempty"`
	Data   []*Profile `json:"data,omitempty"`
}

type ErrResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type GenderizeResponse struct {
	Name        string  `json:"name"`
	Gender      *string `json:"gender"`
	Probability float64 `json:"probability"`
	Count       int     `json:"count"`
	CountryID   *string `json:"country_id,omitempty"`
}

type AgifyResponse struct {
	Count     int    `json:"count"`
	Name      string `json:"name"`
	Age       *int   `json:"age"`
	CountryID string `json:"country_id,omitempty"`
}

type NationalizeResponse struct {
	Count   int       `json:"count"`
	Name    string    `json:"name"`
	Country []Country `json:"country"`
}

type Country struct {
	CountryID   string  `json:"country_id"`
	Probability float64 `json:"probability"`
}

func (s AgifyResponse) GetAgeGroup() string {
	age := *s.Age
	switch {
	case age <= 12:
		return "child"
	case age <= 19:
		return "teenager"
	case age <= 59:
		return "adult"
	default:
		return "senior"
	}
}

func (s NationalizeResponse) GetMostProbableNationality() Country {
	a := s.Country
	mostProb := Country{}
	for _, v := range a {
		if mostProb.Probability < v.Probability {
			mostProb = v
		}
	}
	return mostProb
}
