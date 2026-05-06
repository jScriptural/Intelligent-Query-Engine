package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"intelliqe/internal/models"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store interface {
	GetProfiles(ctx context.Context, q url.Values, page, limit int) ([]*models.Profile, int, error)
	SaveProfile(ctx context.Context, p *models.Profile) error
	GetProfileByName(ctx context.Context, name string) (*models.Profile, error)
	GetProfileByID(ctx context.Context, id string) (*models.Profile, error)
	DeleteProfileByID(ctx context.Context, id string) error
	GetCountryName(ISOcode string) (string, bool)
	UpsertUserSession(ctx context.Context, u *models.UserProfile, tk string) error
	RevokeTokenByID(ctx context.Context, uid string) error
	GetUserProfileByID(ctx context.Context, id string) (*models.UserProfile, error)
	GetToken(ctx context.Context, id string) (*models.RefreshToken, error)
	UpdateUserTokenByID(ctx context.Context, userID, token string) error
	GetUserProfileByGithubID(ctx context.Context, githubID string) (*models.UserProfile, error)
}

type Service struct {
	store  Store
	client *http.Client
}

func NewService(s Store) *Service {
	return &Service{
		store:  s,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

var (
	constraint = map[string][]string{
		"gender":    []string{"male", "female"},
		"age_group": []string{"senior", "adult", "teenager", "child"},
		"sort_by":   []string{"age", "created_at", "gender_probability"},
		"order":     []string{"asc", "desc"},
	}
	supportedFilters = []string{
		"min_age", "max_age", "gender",
		"min_gender_probability", "country_id",
		"age_group", "min_country_probability",
		"sort_by", "order", "page", "limit",
		"country_name",
	}
	keywordsMap = map[string]string{
		"young":     "min_age=16&max_age=24",
		"male":      "gender=male",
		"males":     "gender=male",
		"boy":       "gender=male&max_age=18",
		"boys":      "gender=male&max_age=18",
		"female":    "gender=female",
		"females":   "gender=female",
		"girl":      "gender=female&max_age=18",
		"girls":     "gender=female&max_age=18",
		"adult":     "age_group=adult",
		"adults":    "age_group=adult",
		"teenager":  "age_group=teenager",
		"teenagers": "age_group=teenager",
		"senior":    "age_group=senior",
		"seniors":   "age_group=senior",
		"elder":     "age_group=senior",
		"elderly":   "age_group=senior",
		"elders":    "age_group=senior",
		"child":     "age_group=child",
		"children":  "age_group=child",
		"kid":       "age_group=child",
		"kids":      "age_group=child",
	}
)

func (s *Service) Filter(ctx context.Context, q url.Values) ([]*models.Profile, int, error) {

	page, limit, err := s.ValidateQuery(q)
	if err != nil {
		return nil, 0, fmt.Errorf("Filter: %w", err)
	}

	p, total, err := s.store.GetProfiles(ctx, q, page, limit)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, 0, fmt.Errorf("Filter: %w", err)
	}

	if p == nil {
		return nil, 0, fmt.Errorf("Filter: %w", models.ErrNotFound)
	}

	return p, total, nil
}

func (s *Service) NLProcessor(ctx context.Context, query url.Values, nl string) ([]*models.Profile, int, error) {

	nl = strings.ToLower(nl)
	fields := strings.Fields(nl)
	if len(fields) == 0 {
		return s.Filter(ctx, query)
	}

	fmt.Printf("fields: %#v\n", fields)

	var sb strings.Builder
	for k, v := range keywordsMap {
		if slices.Contains(fields, k) {
			sb.WriteString(v + "&")
		}
	}

	re := regexp.MustCompile(`(above|over)\s*?(\d+)`)
	if m := re.FindStringSubmatch(nl); m != nil {
		sb.WriteString(fmt.Sprintf("min_age=%s&", m[2]))
	}

	re = regexp.MustCompile(`(below|under)\s*?(\d+)`)
	if m := re.FindStringSubmatch(nl); m != nil {
		sb.WriteString(fmt.Sprintf("max_age=%s&", m[2]))

	}

	re = regexp.MustCompile(`between\s*?(\d+)\s*?and\s*?(\d+)`)
	if m := re.FindStringSubmatch(nl); m != nil {
		sb.WriteString(fmt.Sprintf("min_age=%s&max_age=%s&", m[1], m[2]))
	}

	re = regexp.MustCompile(`from\s*?([a-zA-Z-]+)`)
	if m := re.FindStringSubmatch(nl); m != nil {
		sb.WriteString(fmt.Sprintf("country_name=%s&", m[1]))
	}

	if sb.String() == "" {
		return nil, 0, fmt.Errorf("NLProcessor: %w", models.ErrUnInterpretable)
	}

	rawQuery := strings.Trim(sb.String(), "&")
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("NLProcessor: %w", models.ErrUnInterpretable)
	}

	for k, v := range q {
		query.Set(k, v[0])
	}

	return s.Filter(ctx, query)
}

func (s *Service) GetOrCreateProfile(ctx context.Context, name string) (*models.Profile, bool, error) {
	p, err := s.store.GetProfileByName(ctx, name)
	if err == nil {
		return p, false, nil
	}

	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, false, err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	var (
		gRes models.GenderizeResponse
		aRes models.AgifyResponse
		nRes models.NationalizeResponse
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := s.fetchGender(ctx, name, &gRes); err != nil {
			errChan <- fmt.Errorf("genderize: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := s.fetchAge(ctx, name, &aRes); err != nil {
			errChan <- fmt.Errorf("agify: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := s.fetchNation(ctx, name, &nRes); err != nil {
			errChan <- fmt.Errorf("nationalize: %w", err)
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			log.Printf("GetOrCreateProfile: %v", err)
			return nil, false, fmt.Errorf("GetOrCreateProfile: %w", models.Err502)
		}
	}

	if len(nRes.Country) == 0 || aRes.Age == nil || gRes.Gender == nil || gRes.Count == 0 {
		return nil, false, fmt.Errorf("GetOrCreateProfile: %w", models.Err502)
	}

	prof := s.assembleProfile(name, &gRes, &aRes, &nRes)

	if err := s.store.SaveProfile(ctx, prof); err != nil {
		log.Printf("GetOrCreateProfile: %v", err)
		return nil, false, fmt.Errorf("GetOrCreateProfile: %w", err)
	}

	return prof, true, nil
}

func (s *Service) RetrieveProfileByID(ctx context.Context, id string) (*models.Profile, error) {

	p, err := s.store.GetProfileByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("RetrieveProfileByID: %w", err)
	}

	return p, nil
}

func (s *Service) DeleteProfile(ctx context.Context, id string) error {
	err := s.store.DeleteProfileByID(ctx, id)

	if err != nil {
		return fmt.Errorf("DeleteProfile: %w", err)
	}

	return nil
}

func (s *Service) GetAuthCredential(ctx context.Context, gtc models.GitTmpCode) (*models.AuthCredential, error) {

	url := "https://github.com/login/oauth/access_token"

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	val := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          gtc.Code,
		"code_verifier": gtc.Verifier,
	}

	postData, _ := json.Marshal(val)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewBuffer(postData),
	)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("GetAuthCredential: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("GetAuthCredential: %w", models.Err502)
	}
	defer res.Body.Close()

	ghRes := models.GithubTokenResponse{}
	err = json.NewDecoder(res.Body).Decode(&ghRes)
	if err != nil {
		return nil, fmt.Errorf("GetAuthCredemtial: %w", err)
	}

	log.Printf("ghRes: %#v", ghRes)

	if ghRes.Error != "" {
		log.Println(ghRes.ErrorDescription)
		return nil, fmt.Errorf("GetAuthCredential: %w", models.Err502)
	}

	//GET GITHUB PROFILE

	req, err = http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.github.com/user",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAuthCredential: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ghRes.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "insighta-lab+")

	res, err = s.client.Do(req)
	if err != nil || res == nil || res.StatusCode != http.StatusOK {
		log.Println(err)
		if res != nil {
			log.Println(res.Status)
		}
		return nil, fmt.Errorf("GetAuthCredential: %w", models.Err502)
	}
	defer res.Body.Close()

	ghProfile := models.GithubProfile{}
	err = json.NewDecoder(res.Body).Decode(&ghProfile)
	if err != nil {
		return nil, fmt.Errorf("GetAuthCredemtial: %w", err)
	}

	log.Printf("github profile: %#v", ghProfile)
	if ghProfile.Email == "" {
		email, err := s.fetchPrimaryEmail(ctx, ghRes.AccessToken)
		if err == nil {
			ghProfile.Email = email
		}
	}

	u, err := s.store.GetUserProfileByGithubID(ctx, strconv.FormatInt(ghProfile.GithubID, 10))
	if err != nil {
		if errors.Is(err, models.ErrNoUser) {
			log.Printf("No user with github_id: %d", ghProfile.GithubID)
		} else {
			return nil, fmt.Errorf("GetAuthCredential: %w", err)
		}
	}
	role := "analyst"

	if u != nil {
		role = u.Role
	}

	log.Println("Creating user profile")
	id, _ := uuid.NewV7()
	user := models.UserProfile{
		ID:          id,
		GithubID:    strconv.FormatInt(ghProfile.GithubID, 10),
		Username:    ghProfile.Username,
		Email:       ghProfile.Email,
		AvatarURL:   ghProfile.AvatarURL,
		Role:        role,
		IsActive:    true,
		LastLoginAt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}

	authCred, err := s.GenerateCredential(user.ID, user.Username, role)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("GetAuthCredential: %w", err)
	}

	log.Println("Saving user and refresh token to db")
	if err := s.store.UpsertUserSession(ctx, &user, authCred.RefreshToken); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("GetAuthCredential: %w", models.ErrDeadlineExceeded)
		}

		return nil, fmt.Errorf("GetAuthCredential: %w", err)
	}

	return authCred, nil

}

func (s *Service) RevokeToken(ctx context.Context, u *models.UserID) error {
	err := s.store.RevokeTokenByID(ctx, u.UserID)

	if err != nil {
		return err
	}

	return nil
}

func (s *Service) UpdateUserToken(ctx context.Context, id, token string) error {

	err := s.store.UpdateUserTokenByID(ctx, id, token)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) ValidateToken(ctx context.Context, tk string, claims *models.RefreshClaims) (*models.UserProfile, error) {

	secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))
	token, err := jwt.ParseWithClaims(tk, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, models.ErrUnauthorized
		}
		return secretKey, nil
	})

	if err != nil || !token.Valid {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, models.ErrExpiredToken
		}
		return nil, models.ErrUnauthorized
	}

	former, err := s.store.GetToken(ctx, claims.UserID.String())
	if err != nil {
		return nil, fmt.Errorf("ValidateToken: %w", err)
	}
	if former.Token != tk {
		return nil, fmt.Errorf("ValidateToken: token mismatch: %w", models.ErrUnauthorized)
	}

	user, err := s.store.GetUserProfileByID(ctx, claims.UserID.String())
	if err != nil {
		return nil, fmt.Errorf("ValidateToken: %w", err)
	}

	return user, nil
}

/*****************************************
*                                        *
*            HELPER FUNCS                *
*                                        *
******************************************/

func (s *Service) ValidateQuery(q url.Values) (int, int, error) {

	if len(q) == 0 || (!q.Has("page") && !q.Has("limit")) {
		q.Set("page", "1")
		q.Set("limit", "10")
	}

	if q.Has("order") && !q.Has("sort_by") {
		q.Set("sort_by", "id")
	}

	if q.Has("sort_by") && !q.Has("order") {
		q.Set("order", "DESC")
	}

	if q.Has("limit") && !q.Has("page") {
		q.Set("page", "1")
	}

	if q.Has("page") && !q.Has("limit") {
		q.Set("limit", "10")
	}

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return 0, 0, models.ErrInvalidParam
	}

	if limit > 50 {
		q.Set("limit", "50")
		limit = 50
	}

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil {
		return 0, 0, models.ErrInvalidParam
	}

	for k, v := range q {
		if !slices.Contains(supportedFilters, k) {
			return 0, 0, models.ErrInvalidParam
		}
		if len(v) == 0 {
			return 0, 0, models.ErrEmptyParam
		}

		if c, ok := constraint[k]; ok {
			val := strings.ToLower(v[0])
			if !slices.Contains(c, val) {
				return 0, 0, models.ErrInvalidParam
			}
		}
	}

	return page, limit, nil

}

func (s *Service) fetchNation(ctx context.Context, name string, nRes *models.NationalizeResponse) error {
	u, err := url.Parse("https://api.nationalize.io")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("name", name)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nationalize api returned status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(nRes); err != nil {
		return fmt.Errorf("nationalize: %w", err)
	}

	return nil
}

func (s *Service) fetchGender(ctx context.Context, name string, gRes *models.GenderizeResponse) error {
	u, err := url.Parse("https://api.genderize.io")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("name", name)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("genderize api returned status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(gRes); err != nil {
		return fmt.Errorf("genderize: %w", err)
	}

	return nil
}

func (s *Service) fetchAge(ctx context.Context, name string, aRes *models.AgifyResponse) error {

	u, err := url.Parse("https://api.agify.io")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("name", name)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agify api returned status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(aRes); err != nil {
		return fmt.Errorf("agify: %w", err)
	}

	return nil
}

func (s *Service) assembleProfile(name string, gRes *models.GenderizeResponse, aRes *models.AgifyResponse, nRes *models.NationalizeResponse) *models.Profile {
	p := models.Profile{}

	p.ID, _ = uuid.NewV7()
	p.Name = name
	p.Gender = *gRes.Gender
	p.GenderProbability = gRes.Probability
	p.Age = *aRes.Age
	p.AgeGroup = aRes.GetAgeGroup()

	c := nRes.GetMostProbableNationality()
	cn, ok := s.store.GetCountryName(c.CountryID)
	if !ok {
		cn = ""
	}
	p.CountryID = c.CountryID
	p.CountryName = cn
	p.CountryProbability = c.Probability
	p.CreatedAt = time.Now().UTC()

	return &p
}

func (s *Service) GenerateCredential(id uuid.UUID, username, role string) (*models.AuthCredential, error) {
	log.Printf("GenerateCred: id - %v", id.String())
	secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))

	accessClaims := models.AccessClaims{
		UserID:   id,
		Role:     role,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(3 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   id.String(),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(secretKey)
	if err != nil {
		return nil, err
	}

	refreshClaims := models.RefreshClaims{
		UserID: id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(120 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   id.String(),
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(secretKey)
	if err != nil {
		return nil, err
	}

	return &models.AuthCredential{
		Status:       "success",
		AccessToken:  accessToken,
		RefreshToken: refreshToken}, nil
}

func (s *Service) fetchPrimaryEmail(ctx context.Context, token string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(res.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Verified && e.Primary {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary verified email found")
}
