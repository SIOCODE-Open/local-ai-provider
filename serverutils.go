package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOpenAIError(w http.ResponseWriter, status int, message string, errType string, param interface{}, code interface{}) {
	if errType == "" {
		errType = "invalid_request_error"
		if status == http.StatusUnauthorized {
			errType = "authentication_error"
		} else if status == http.StatusTooManyRequests {
			errType = "rate_limit_error"
		} else if status >= 500 {
			errType = "api_error"
		}
	}
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errType,
			"param":   param,
			"code":    code,
		},
	}
	writeJSON(w, status, payload)
}

func writeVertexError(w http.ResponseWriter, status int, message string, errType string, code string) {
	if errType == "" {
		errType = "api_error"
	}
	statusStr := "INVALID_ARGUMENT"
	switch status {
	case http.StatusUnauthorized:
		statusStr = "UNAUTHENTICATED"
	case http.StatusNotFound:
		statusStr = "NOT_FOUND"
	case http.StatusTooManyRequests:
		statusStr = "RESOURCE_EXHAUSTED"
	default:
		if status >= 500 {
			statusStr = "INTERNAL"
		}
	}
	errObj := map[string]interface{}{
		"code":    status,
		"message": message,
		"status":  statusStr,
		"type":    errType,
	}
	if code != "" {
		errObj["error_code"] = code
	}
	writeJSON(w, status, map[string]interface{}{
		"error": errObj,
	})
}

func checkOpenAIAuth(r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token := strings.TrimSpace(auth[7:])
		return token != ""
	}
	return false
}

func checkVertexAuth(r *http.Request) bool {
	if checkOpenAIAuth(r) {
		return true
	}
	if strings.TrimSpace(r.Header.Get("x-goog-api-key")) != "" {
		return true
	}
	if strings.TrimSpace(r.URL.Query().Get("key")) != "" {
		return true
	}
	return false
}

func accessLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		AppContext.mu.RLock()
		allowedOrigins := AppContext.Config.AllowedOrigins
		AppContext.mu.RUnlock()

		if allowedOrigins == "*" || allowedOrigins == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		reqHeaders := r.Header.Get("Access-Control-Request-Headers")
		if reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-goog-api-key")
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
