package service

import (
	"context"
	"fmt"
	"intelliqe/internal/models"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Store interface {
	GetProfiles(ctx context.Context, q url.Values, page, limit int) ([]*models.Profile, int, error)
}

type Service struct {
	store Store
}

func NewService(s Store) *Service {
	return &Service{
		store: s,
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
		"female":    "gender=female",
		"females":   "gender=female",
		"adult":     "age_group=adult",
		"adults":    "age_group=adult",
		"teenager":  "age_group=teenager",
		"teenagers": "age_group=teenager",
		"senior":    "age_group=senior",
		"seniors":   "age_group=senior",
		"child":     "age_group=child",
		"children":  "age_group=child",
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
		sb.WriteString(fmt.Sprintf("country_name=%s&",m[1]));
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
