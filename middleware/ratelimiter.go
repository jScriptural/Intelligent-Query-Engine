package middleware

import (
	"encoding/json"
	"intelliqe/internal/models"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Token byte
type Bucket struct {
	C           chan Token
	AccessTime  time.Time
	WindowStart time.Time
	isClose     chan byte
}
type BucketGroup map[string]*Bucket

func NewBucket(size int, d time.Duration) *Bucket {
	b := &Bucket{
		C:           make(chan Token, size),
		AccessTime:  time.Now().UTC(),
		WindowStart: time.Now().UTC(),
		isClose:     make(chan byte),
	}
	go func(b *Bucket) {
		for {
			select {
			case <-b.isClose:
				return
			default:
				go func() {
					for i := 0; i < size; i++ {
						if len(b.C) == size {
							break
						}
						select {
						case <-b.isClose:
							return
						default:
							b.C <- 'a'
						}
					}
				}()
				<-time.After(d)
				b.WindowStart = time.Now().UTC()
			}
		}
	}(b)
	return b
}

func NewBucketGroup() BucketGroup {
	bg := BucketGroup{}
	go func(bgrp BucketGroup) {
		for {
			time.Sleep(time.Hour)
			for k, bucket := range bgrp {
				at := bucket.AccessTime
				now := time.Now().UTC()
				if now.After(at.Add(time.Hour)) {
					close(bucket.isClose)
					delete(bgrp, k)
					log.Printf("size of BucketGroup: %v", len(bgrp))
				}
			}
		}
	}(bg)
	return bg
}

func (bg BucketGroup) Get(key string) (*Bucket, bool) {
	b, ok := bg[key]
	if !ok {
		return nil, ok
	}
	b.AccessTime = time.Now().UTC()

	return b, ok
}

func (bg BucketGroup) Set(key string, val *Bucket) {
	bg[key] = val
}

var Bgrp = NewBucketGroup()

func NewRateLimiter(size int, d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := ""
			u, ok := FromContext(r.Context())
			if ok {
				id = u.UserID.String()
			} else {
				id = strings.Split(r.RemoteAddr, ":")[0]
			}
			b, exists := Bgrp.Get(id)
			if !exists {
				b = NewBucket(size, d)
				Bgrp.Set(id, b)
			}

			w.Header().Set(
				"X-Ratelimit-Limit",
				strconv.Itoa(size),
			)
			select {
			case <-b.C:
				w.Header().Set(
					"X-Ratelimit-Remaining",
					strconv.Itoa(len(b.C)),
				)
				next.ServeHTTP(w, r)
			case <-time.After(100 * time.Microsecond):
				w.Header().Set(
					"X-Ratelimit-Retry-After",
					time.Until(b.WindowStart.Add(d)).String(),
				)
				w.Header().Set(
					"X-Ratelimit-Remaining",
					strconv.Itoa(len(b.C)),
				)
				w.Header().Set(
					"Content-Type",
					"application/json",
				)
				w.WriteHeader(http.StatusTooManyRequests)
				if err := json.NewEncoder(w).Encode(models.ErrResponse{Status: "error", Message: "Too many requests"}); err != nil {
					log.Printf("NewRateLimiter: %v", err)
				}
				return
			} //switch

		})
	}
}
