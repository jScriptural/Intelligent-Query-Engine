package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"intelliqe/internal/models"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type key string

var userKey key

func NewContext(ctx context.Context, u *models.UserInfo) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func FromContext(ctx context.Context) (*models.UserInfo, bool) {
	userInfo, ok := ctx.Value(userKey).(*models.UserInfo)

	return userInfo, ok
}

func CORS(h http.Handler, config map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("clientIP: %v\n", getRealClientIP(r))
		log.Printf("URI: %v", r.URL.RequestURI())
		log.Printf("path: %v", r.URL.Path)
		log.Printf("Method: %v", r.Method)

		for k, v := range config {
			w.Header().Set(k, v)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Header.Get("X-API-Version")
		if val == "" || val != "1" {
			w.Header().Set("Content", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(models.ErrResponse{Status: "error", Message: "API version header required"}); err != nil {
				log.Printf("Auth: %v", err)
			}
			return

		}

		accessToken := getBearerToken(r)
		if accessToken == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(models.ErrResponse{Status: "error", Message: "Authorization required."}); err != nil {
				log.Printf("Auth: %v", err)
			}
			return
		}

		secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))
		claims := &models.AccessClaims{}

		token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			return secretKey, nil
		})

		if err != nil || !token.Valid {
			log.Printf("Auth: Invalid token- %v:%v", token, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			if err := json.NewEncoder(w).Encode(models.ErrResponse{Status: "error", Message: "Authorization required"}); err != nil {
				log.Printf("Auth: %v", err)
			}
			return
		}

		userInfo := models.UserInfo{
			UserID:   claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
		}

		ctx := NewContext(r.Context(), &userInfo)
		b, _ := json.MarshalIndent(userInfo, "", " ")
		log.Printf("usr: %s", b)
		next.ServeHTTP(w, r.WithContext(ctx))
	})

}

func ResponseTimer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now().UTC()
		defer func(t time.Time) {
			log.Printf(
				"Response time: %v",
				time.Since(t).String(),
			)
		}(t)

		next.ServeHTTP(w, r)
	})

}

func getBearerToken(r *http.Request) string {
	autho := r.Header.Get("Authorization")
	if autho == "" {
		return ""
	}

	re := regexp.MustCompile(`Bearer\s+?(.*)\s*?$`)

	m := re.FindStringSubmatch(autho)
	if m == nil {
		return ""
	}

	return m[1]
}

func getRealClientIP(r *http.Request) string {
	headers := []string{
		"X-REAL-IP",
		"X-Forwarded-For",
		"X-Client-IP",
		"CF-Connecting-IP",
		"X-Forwarded",
		"Forwarded-For",
	}

	for k,v := range r.Header {
		log.Printf("%v:%v",k,v)
	}
	for _, header := range headers {
		if v := r.Header.Get(header); v != "" {
			ips := strings.Split(v, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if ip != "" {
					return ip
				}
			}
		}
	}

	return strings.Split(r.RemoteAddr, ":")[0]
}
