package billing

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteError sends a JSON error with machine-readable code and logs to console.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	log.Printf("[API] %d %s code=%s", status, message, code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  code,
	})
}
