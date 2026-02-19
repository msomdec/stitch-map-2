package handler

import (
	"encoding/json"
	"net/http"
)

// HandleHealthz responds with a 200 OK and a JSON body indicating the server is healthy.
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
