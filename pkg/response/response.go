package response

import (
	"encoding/json"
	"net/http"
)

// Envelope wraps every successful response body: {"data": ...}.
type Envelope struct {
	Data any `json:"data"`
}

// ErrorBody wraps every error response body: {"error": {...}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Data(w http.ResponseWriter, status int, data any) {
	JSON(w, status, Envelope{Data: data})
}

func Error(w http.ResponseWriter, status int, detail ErrorDetail) {
	JSON(w, status, ErrorBody{Error: detail})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
