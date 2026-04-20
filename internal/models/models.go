package models

import (
	"github.com/google/uuid"
	"time"
)

type Profile struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Gender             string    `json:"gender"`
	GenderProbability  float64   `json:"gender_probability"`
	Age                int       `json:"age"`
	AgeGroup           string    `json:"age_group"`
	CountryID          string    `json:"country_id"`
	CountryName        string    `json:"country_name"`
	CountryProbability float64   `json:"country_probability"`
	CreatedAt          time.Time `json:"created_at"`
}

type Response struct {
	Status string  `json:"status"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
	Data   Profile `json:"data"`
}

type ErrResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
