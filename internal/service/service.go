package service

import (
	"context"
	"fmt"
	"intelliqe/internal/models"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type Store interface {
	GetProfiles(ctx context.Context, q url.Values,page,limit int) ([]*models.Profile,int,error)
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
	constraint =  map[string][]string{
		"gender": []string{"male","female"},
		"age_group": []string{"senior","adult","teenager","child"},
		"sort_by": []string{"age","created_at","gender_probability"},
		"order": []string{"asc","desc"},
	}
	supportedFilters = []string{
		"min_age","max_age","gender",
		"min_gender_probability","country_id",
		"age_group","min_country_probability",
		"sort_by","order","page","limit",
	}
)

func (s *Service) Filter(ctx context.Context, q url.Values) ([]*models.Profile,int,error) {
	if len(q) == 0  || (!q.Has("page") && !q.Has("limit")){
		q.Set("page", "1")
		q.Set("limit", "10")
	}


	if q.Has("order") && !q.Has("sort_by"){
		q.Set("sort_by","id");
	}

	if q.Has("sort_by") && !q.Has("order"){
		q.Set("order","DESC");
	}

	if q.Has("limit") && !q.Has("page") {
		q.Set("page","1");
	}

	if q.Has("page") && !q.Has("limit") {
		q.Set("limit","10");
	}

	limit, err := strconv.Atoi(q.Get("limit"));
	if err != nil {
		return nil,0,fmt.Errorf("Filter: %w",models.ErrInvalidParam);
	}

	if limit > 50 {
		q.Set("limit","50")
		limit = 50;
	}

	page, err := strconv.Atoi(q.Get("page"));
	if err != nil {
		return nil,0,fmt.Errorf("Filter: %w",models.ErrInvalidParam);
	}


	for k, v := range q {
		if !slices.Contains(supportedFilters,k){
			return nil,0, fmt.Errorf("Filter: %w", models.ErrInvalidParam)
		}
		if len(v) == 0 {
			return nil,0,fmt.Errorf("Filter: %w", models.ErrEmptyParam)
		}

		if c,ok := constraint[k]; ok {
			val := strings.ToLower(v[0]);
			if !slices.Contains(c,val) {
				return nil,0,fmt.Errorf("Filter: %w",models.ErrInvalidParam);
			}
		}	
	}


	p,total,err := s.store.GetProfiles(ctx,q,page,limit)
	if err != nil {
		fmt.Printf("Error: %v\n",err);
		return nil,0, fmt.Errorf("Filter: %w", err)
	}

	if p == nil {
		return nil,0, fmt.Errorf("Filter: %w", models.ErrNotFound)
	}

	return p,total, nil
}
