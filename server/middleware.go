package server

import (
	"net/http"
	"strings"
)

// UseContentNegotiation provides JSON:API content negotiation middleware.
// It validates Content-Type and Accept headers according to JSON:API specification.
// Returns 415 Unsupported Media Type for invalid Content-Type on requests with body.
// Returns 406 Not Acceptable for invalid Accept headers.
func UseContentNegotiation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const jsonAPIContentType = "application/vnd.api+json"

		// Check Content-Type for requests with body
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			contentType := r.Header.Get("Content-Type")
			if contentType != "" && contentType != jsonAPIContentType {
				http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
				return
			}
		}

		// Check Accept header
		accept := r.Header.Get("Accept")
		if accept != "" && accept != "*/*" && !strings.Contains(accept, jsonAPIContentType) {
			http.Error(w, "Not Acceptable", http.StatusNotAcceptable)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// UseRequestContextResolver creates middleware that resolves request context
// information using the provided resolver and makes it available to downstream
// handlers. If context resolution fails, it returns an HTTP 500 error.
func UseRequestContextResolver(next http.Handler, resolver RequestContextResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestContext, err := resolver.ResolveRequestContext(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx := SetRequestContext(r.Context(), requestContext)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DefaultContextResolver is the default implementation of [RequestContextResolver].
// It extracts JSON:API context information from URL path parameters using Go's
// standard HTTP router path value extraction. This resolver works with the
// standard JSON:API URL patterns.
type DefaultContextResolver struct{}

// ResolveRequestContext implements the [RequestContextResolver] interface.
// It extracts resource type, ID, and relationship information from the HTTP
// request URL path parameters and constructs a RequestContext with the parsed
// information.
func (DefaultContextResolver) ResolveRequestContext(r *http.Request) (*RequestContext, error) {
	var (
		id           = r.PathValue("id")
		rtype        = r.PathValue("type")
		relationship = r.PathValue("relationship")
		related      = r.PathValue("related")
	)

	rc := RequestContext{
		ResourceID:            id,
		ResourceType:          rtype,
		Relationship:          relationship,
		FetchRelatedResources: related != "",
	}

	if rc.FetchRelatedResources {
		rc.Relationship = related
	}

	return &rc, nil
}
