package exhttp

import (
	"encoding/json"
	"net/http"
)

var AutoAllowCORS = true

func WriteJSONResponse(w http.ResponseWriter, httpStatusCode int, jsonData any) {
	if AutoAllowCORS {
		AddCORSHeaders(w)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(jsonData)
}

func WriteJSONData(w http.ResponseWriter, httpStatusCode int, data []byte) {
	if AutoAllowCORS {
		AddCORSHeaders(w)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_, _ = w.Write(data)
}

func WriteEmptyJSONResponse(w http.ResponseWriter, httpStatusCode int) {
	if AutoAllowCORS {
		AddCORSHeaders(w)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_, _ = w.Write([]byte("{}"))
}
