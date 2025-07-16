package server

import (
	"context"
	"net/http"
	"strings"
)

// RequestContext contains parsed information from an HTTP request that is relevant
// to JSON:API resource operations. It provides structured access to resource
// identifiers, types, and relationship information extracted from the request URL.
type RequestContext struct {
	ResourceID            string // The ID of the requested resource
	ResourceType          string // The type of the requested resource
	Relationship          string // The name of the requested relationship
	FetchRelatedResources bool   // Whether to fetch related resources instead of relationship linkage
}

// GetFields returns the sparse fieldset for a resource type from the request query parameters.
// It parses the fields[type]=field1,field2 query parameter format.
// Returns an empty slice if no fields are specified for the resource type.
func (r RequestContext) GetFields(req *http.Request, resourceType string) []string {
	fieldParam := req.URL.Query().Get("fields[" + resourceType + "]")
	if fieldParam == "" {
		return []string{}
	}

	fields := strings.Split(fieldParam, ",")
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}
	return fields
}

// ShouldInclude checks if a relationship path should be included based on the
// include query parameter. It parses the include=relationship1,relationship2 format
// and returns true if the specified path is found in the include list.
func (r RequestContext) ShouldInclude(req *http.Request, path string) bool {
	includeParam := req.URL.Query().Get("include")
	if includeParam == "" {
		return false
	}

	includes := strings.Split(includeParam, ",")
	for _, include := range includes {
		if strings.TrimSpace(include) == path {
			return true
		}
	}
	return false
}

// GetPageParam returns a pagination parameter value from the request query parameters.
// It parses the page[key]=value query parameter format.
// Returns an empty string if the parameter is not found.
func (r RequestContext) GetPageParam(req *http.Request, key string) string {
	return req.URL.Query().Get("page[" + key + "]")
}

// GetFilterParam returns a filter parameter value from the request query parameters.
// It parses the filter[key]=value query parameter format.
// Returns an empty string if the parameter is not found.
func (r RequestContext) GetFilterParam(req *http.Request, key string) string {
	return req.URL.Query().Get("filter[" + key + "]")
}

// GetSort returns the sort fields from the request query parameters.
// It parses the sort=field1,-field2 query parameter format.
// Returns an empty slice if no sort parameter is specified.
func (r RequestContext) GetSort(req *http.Request) []string {
	sortParam := req.URL.Query().Get("sort")
	if sortParam == "" {
		return []string{}
	}

	sorts := strings.Split(sortParam, ",")
	for i, sort := range sorts {
		sorts[i] = strings.TrimSpace(sort)
	}
	return sorts
}

// RequestContextResolver defines the interface for extracting [RequestContext]
// information from HTTP requests. Implementations should parse the request URL
// and headers to populate the context with relevant JSON:API resource information.
type RequestContextResolver interface {
	// ResolveRequestContext extracts JSON:API context information from the HTTP request.
	// It returns a populated RequestContext or an error if the request cannot be parsed.
	ResolveRequestContext(r *http.Request) (*RequestContext, error)
}

// requestContextKey is used as a key for storing [RequestContext] in the request context.
// It uses an empty struct to ensure uniqueness and prevent key collisions.
type requestContextKey struct{}

// SetRequestContext stores a [RequestContext] in the provided context and returns
// the new context. This is typically used in middleware to make request context
// information available to downstream handlers.
func SetRequestContext(parent context.Context, value *RequestContext) context.Context {
	return context.WithValue(parent, requestContextKey{}, value)
}

// GetRequestContext retrieves a [RequestContext] from the provided context.
// It returns the RequestContext and a boolean indicating whether the context
// was found. If no context is found, it returns nil and false.
func GetRequestContext(ctx context.Context) (*RequestContext, bool) {
	value := ctx.Value(requestContextKey{})
	if value == nil {
		return nil, false
	}
	return value.(*RequestContext), true
}
