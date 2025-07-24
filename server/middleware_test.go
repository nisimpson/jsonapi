package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUseContentNegotiation(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		accept         string
		expectedStatus int
	}{
		{
			name:           "GET request with valid Accept header",
			method:         http.MethodGet,
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET request with invalid Accept header",
			method:         http.MethodGet,
			accept:         "application/json",
			expectedStatus: http.StatusNotAcceptable,
		},
		{
			name:           "GET request with no Accept header",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK, // Default behavior should allow this
		},
		{
			name:           "POST request with valid Content-Type",
			method:         http.MethodPost,
			contentType:    "application/vnd.api+json",
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request with invalid Content-Type",
			method:         http.MethodPost,
			contentType:    "application/json",
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "POST request with no Content-Type",
			method:         http.MethodPost,
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "POST request with Content-Type parameters",
			method:         http.MethodPost,
			contentType:    "application/vnd.api+json; charset=utf-8",
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusUnsupportedMediaType, // JSON:API spec requires no parameters
		},
		{
			name:           "PATCH request with valid Content-Type",
			method:         http.MethodPatch,
			contentType:    "application/vnd.api+json",
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE request with no Content-Type",
			method:         http.MethodDelete,
			accept:         "application/vnd.api+json",
			expectedStatus: http.StatusOK, // DELETE doesn't require Content-Type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that always returns 200 OK and sets the Content-Type header
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
			})

			// Wrap the test handler with content negotiation middleware
			handler := UseContentNegotiation(testHandler)

			// Create a test request
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// If the request was successful, check that the response has the correct Content-Type
			if rr.Code == http.StatusOK {
				assert.Equal(t, "application/vnd.api+json", rr.Header().Get("Content-Type"))
			}
		})
	}
}
