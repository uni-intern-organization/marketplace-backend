package middleware

import (
	"log"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging logs every HTTP request and highlights client/server errors.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		switch {
		case sw.status >= 500:
			log.Printf("[HTTP] %s %s -> %d (%s) SERVER_ERROR", r.Method, r.URL.Path, sw.status, dur)
		case sw.status >= 400:
			log.Printf("[HTTP] %s %s -> %d (%s) client_error", r.Method, r.URL.Path, sw.status, dur)
		default:
			log.Printf("[HTTP] %s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, dur)
		}
	})
}
