package models

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
)

var (
	ErrNotFound         = errors.New("Not found")
	ErrInvalidParam     = errors.New("Unprocessable entity")
	ErrUnInterpretable  = errors.New("Unable to interpret query")
	ErrEmptyParam       = errors.New("Bad request")
	ErrEmptyBody        = errors.New("Bad request: Empty request body")
	Err502              = errors.New("Bad gateway or upstream error")
	ErrDeadlineExceeded = errors.New("Request timeout")
	ErrUnauthorized     = errors.New("Authorizaton required")
	ErrExpiredToken     = errors.New("Expired refresh token")
	ErrNoSession        = errors.New("No such session")
	ErrNoUser           = errors.New("No User Found")
	ErrForbidden        = errors.New("action is forbidden")
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

type PageLinks struct {
	Self string `json:"self"`
	Next string `json:"next"`
	Prev string `json:"prev"`
}

type UserID struct {
	UserID string `json:"user_id"`
}

type RefreshToken struct {
	Token    string `json:"refresh_token"`
	UserID   string `json:"id,omitempty"`
	GithubID string `json:"github_id,omitempty"`
}

type Response struct {
	Status     string     `json:"status"`
	Page       int        `json:"page,omitempy"`
	Limit      int        `json:"limit,omitempty"`
	Total      int        `json:"total,omitempty"`
	TotalPages int        `json:"total_pages,omitempty"`
	Links      PageLinks  `json:"links,omitempty"`
	Data       []*Profile `json:"data,omitempty"`
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

type GitTmpCode struct {
	Code     string `json:"code"`
	Verifier string `json:"verifier"`
}

type GithubTokenResponse struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	ExpiresIn             int    `json:"expires_in,omitempty"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in,omitempty"`

	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

type AuthCredential struct {
	Status       string `json:"status"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type GithubProfile struct {
	GithubID  int64  `json:"id"`
	Username  string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	GithubID    string    `json:"github_id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"`
	IsActive    bool      `json:"is_active"`
	LastLoginAt time.Time `json:"last_login_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type AccessClaims struct {
	Username string    `json:"username"`
	UserID   uuid.UUID `json:"user_id"`
	Role     string    `json:"role"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
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
