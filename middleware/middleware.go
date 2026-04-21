package middleware

import (
	"net/http"
	"log"
)


func CORS(h http.Handler, config map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("clientIP: %v\n",r.RemoteAddr)
		for k,v := range config {
			w.Header().Set(k,v);
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return;
		}
		h.ServeHTTP(w,r)
	})
}



