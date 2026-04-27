package service

import (
	"context"
	"fmt"
	"intelliqe/internal/models"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"encoding/json"
	"log"
	"errors"
	"sync"
	"github.com/google/uuid"
)

type Store interface {
	GetProfiles(ctx context.Context, q url.Values, page, limit int) ([]*models.Profile, int, error)
	SaveProfile(ctx context.Context, p *models.Profile) error
	GetProfileByName(ctx context.Context, name string) (*models.Profile, error)
	GetProfileByID(ctx context.Context, id string) (*models.Profile, error)
	DeleteProfileByID(ctx context.Context, id string) error
	GetCountryName(ISOcode string) (string,bool)
}

type Service struct {
	store  Store
	client *http.Client
}

func NewService(s Store) *Service {
	return &Service{
		store:  s,
		client: &http.Client{Timeout: 10 * time.Second},
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
	cn,ok := s.store.GetCountryName(c.CountryID);
	if !ok {
		cn = "";
	}
	p.CountryID = c.CountryID
	p.CountryName = cn
	p.CountryProbability = c.Probability
	p.CreatedAt = time.Now().UTC()

	return &p
}
