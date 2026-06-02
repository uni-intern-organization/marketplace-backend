package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// RespondError writes JSON error to client and logs details to stderr (console).
func RespondError(w http.ResponseWriter, status int, message string, err error) {
	RespondErrorWithCode(w, status, "", message, err)
}

// RespondErrorWithCode writes JSON {error, code?} and logs to console.
func RespondErrorWithCode(w http.ResponseWriter, status int, code, message string, err error) {
	if err != nil {
		if code != "" {
			log.Printf("[API] %d %s code=%s: %v", status, message, code, err)
		} else {
			log.Printf("[API] %d %s: %v", status, message, err)
		}
	} else if status >= 400 {
		if code != "" {
			log.Printf("[API] %d %s code=%s", status, message, code)
		} else {
			log.Printf("[API] %d %s", status, message)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"error": message}
	if code != "" {
		body["code"] = code
	}
	_ = json.NewEncoder(w).Encode(body)
}
