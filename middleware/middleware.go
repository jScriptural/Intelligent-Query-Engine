package middleware

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"intelliqe/internal/models"
	"log"
	"net/http"
	"os"
	"regexp"
	//"slices"
	"fmt"
)

func CORS(h http.Handler, config map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("clientIP: %v\n", r.RemoteAddr)
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
		if val == "" || val != "1"{
			log.Printf("val: %#v",val)
			log.Printf("val: %#v",r.Header)
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
			return;
		}

		ctx := context.WithValue(r.Context(), "username", claims.Username)
		ctx = context.WithValue(ctx, "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "role", claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
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

	log.Println("Authomatch: ", m)
	return m[1]
}
