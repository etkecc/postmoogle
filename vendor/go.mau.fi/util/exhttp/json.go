package exhttp

import (
	"encoding/json"
	"net/http"
)

func WriteJSONResponse(w http.ResponseWriter, httpStatusCode int, jsonData any) {
	AddCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_ = json.NewEncoder(w).Encode(jsonData)
}

func WriteJSONData(w http.ResponseWriter, httpStatusCode int, data []byte) {
	AddCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_, _ = w.Write(data)
}

func WriteEmptyJSONResponse(w http.ResponseWriter, httpStatusCode int) {
	AddCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	_, _ = w.Write([]byte("{}"))
}
