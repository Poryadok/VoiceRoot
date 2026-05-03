package middleware

import (
	"net/http"
)

const defaultRequestIDHeader = "X-Request-Id"

// RequestID ensures the response and downstream handler see a request correlation id.
// If the incoming request has no header, generate is called.
func RequestID(generate func() string) func(http.Handler) http.Handler {
	return RequestIDHeader(defaultRequestIDHeader, generate)
}

// RequestIDHeader is like RequestID but uses a custom header name.
func RequestIDHeader(headerName string, generate func() string) func(http.Handler) http.Handler {
	if headerName == "" {
		headerName = defaultRequestIDHeader
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(headerName)
			if id == "" && generate != nil {
				id = generate()
			}
			w.Header().Set(headerName, id)
			r.Header.Set(headerName, id)
			next.ServeHTTP(w, r)
		})
	}
}
