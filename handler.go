package jsonapi

import (
	"context"
	"io"
	"net/http"
	"strconv"
)

// Context represents a parsed JSON:API HTTP request containing information
// about the target resource, relationship, and operation type.
type Context struct {
	ResourceID   string // Unique identifier of the target resource
	ResourceType string // Type of the target resource
	Relationship string // Name of the relationship being accessed
	Related      bool   // Whether this is a request for related resources
}

// requestContextKey is used as a key for storing Request objects in context.Context.
type requestContextKey struct{}

// WithContext creates a new context containing the provided JSON:API request information.
// This allows handlers to access request details without parsing URLs repeatedly.
func WithContext(ctx context.Context, request *Context) context.Context {
	return context.WithValue(ctx, requestContextKey{}, request)
}

// FromContext extracts JSON:API request information from the provided context.
// It returns an empty [Context] if no request information is found, ensuring this function never fails.
func FromContext(ctx context.Context) *Context {
	request, ok := ctx.Value(requestContextKey{}).(*Context)
	if !ok {
		// return an empty request object -- this function should never fail
		// even if a jsonapi request context is missing.
		return &Context{}
	}
	return request
}

// Unmarshal reads the request body and unmarshals JSON:API data into the target.
func (c *Context) Unmarshal(r io.Reader, target interface{}, opts ...Options) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return Unmarshal(body, target, opts...)
}

// UnmarshalRef reads the request body and unmarshals relationship data into the target.
func (c *Context) UnmarshalRef(r io.Reader, name string, target RelationshipUnmarshaler, opts ...Options) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return UnmarshalRef(body, name, target, opts...)
}

// Marshal marshals data into a JSON:API document and writes it to the HTTP response.
// It sets the appropriate Content-Type header and HTTP status code, returning the number
// of bytes written and any marshaling or writing errors.
func (c *Context) Marshal(w http.ResponseWriter, status int, data interface{}, opts ...Options) (n int, err error) {
	return write(w, status, data, opts...)
}

// MarshalRef marshals a specific relationship from a resource into a JSON:API document
// and writes it to the HTTP response. This method is designed for relationship endpoints
// that return relationship data without the parent resource.
//
// The method extracts the specified relationship from the resource and creates a document
// with the relationship data as the primary data, following the JSON:API specification
// for relationship endpoints like GET /articles/1/relationships/author.
//
// It sets the appropriate Content-Type header (application/vnd.api+json) and HTTP status code,
// returning the number of bytes written and any marshaling or writing errors.
//
// Example usage in a relationship handler:
//
//	func getArticleAuthor(w http.ResponseWriter, r *http.Request) {
//		ctx := jsonapi.RequestFromContext(r.Context())
//		article := getArticleByID(ctx.ResourceID)
//
//		_, err := ctx.MarshalRef(w, http.StatusOK, "author", article)
//		if err != nil {
//			// Handle error
//		}
//	}
//
// The relationship must be defined in the resource's [RelationshipMarshaler.Relationships] method,
// otherwise an error is returned during marshaling.
func (c *Context) MarshalRef(w http.ResponseWriter, status int, name string, data RelationshipMarshaler, opts ...Options) (n int, err error) {
	return writeRef(w, status, name, data, opts...)
}

// MarshalErrors creates a JSON:API error document from the provided errors and writes it to the response.
// Each error is converted to a JSON:API error object with the specified HTTP status code.
func (c *Context) MarshalErrors(w http.ResponseWriter, status int, errs ...error) (n int, err error) {
	return writeErrors(w, status, errs...)
}

// RequestResolver defines the interface for parsing HTTP requests into JSON:API request objects.
// Implementations can extract resource information from URLs, headers, or other request components.
type RequestResolver interface {
	// ResolveJSONAPIRequest parses an HTTP request and returns JSON:API request information.
	ResolveJSONAPIRequest(r *http.Request) *Context
}

// Handle creates HTTP middleware that wraps a handler with JSON:API request parsing functionality.
// It uses the provided resolver to extract JSON:API request information from HTTP requests
// and adds this information to the request context for use by downstream handlers.
func Handle(resolver RequestResolver, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := resolver.ResolveJSONAPIRequest(r)
		ctx := WithContext(r.Context(), request)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DefaultRequestResolver implements [RequestResolver] by extracting JSON:API request information
// from URL path parameters using Go 1.22+ ServeMux path value functionality. Typically used
// in conjunction with [DefaultServeMux], this resolver is opinionated on the path variable names
// in incoming request URL paths.
type DefaultRequestResolver struct{}

// ResolveJSONAPIRequest extracts resource information from URL path parameters.
// It expects path parameters named "id", "type", "ref", and "related" to be present in the request.
func (p DefaultRequestResolver) ResolveJSONAPIRequest(r *http.Request) *Context {
	request := &Context{
		ResourceID:   r.PathValue("id"),
		ResourceType: r.PathValue("type"),
		Relationship: r.PathValue("ref"),
	}

	related := r.PathValue("related")
	if related != "" {
		request.Related = true
		request.Relationship = related
	}
	return request
}

// DefaultServeMux creates a pre-configured HTTP ServeMux with JSON:API routing patterns
// and the provided resource handlers. It uses [DefaultRequestResolver] for URL parsing
// and supports all standard JSON:API endpoints including relationships and related resources.
// Additional [Middleware] can be provided and will be applied to the handler chain after
// the JSON:API [Context] parsing is applied.
func DefaultServeMux(handlers map[string]ResourceHandler, middleware ...Middleware) *http.ServeMux {
	var (
		resolver    = DefaultRequestResolver{}
		resourceMux = Handle(resolver, ResourceHandlerMux(handlers))
		handler     = Use(resourceMux, middleware...)
		serveMux    = http.NewServeMux()
	)

	patterns := []string{
		"GET    /{type}/{id}/relationships/{ref}", // get relationship
		"POST   /{type}/{id}/relationships/{ref}", // add to many relationship
		"PATCH  /{type}/{id}/relationships/{ref}", // update relationship
		"DELETE /{type}/{id}/relationships/{ref}", // remove from many relationship
		"GET    /{type}/{id}/{related}",           // get related resource
		"GET    /{type}/{id}",                     // get resource
		"PATCH  /{type}/{id}",                     // update resource
		"DELETE /{type}/{id}",                     // delete resource
		"GET    /{type}",                          // list resources
		"POST   /{type}",                          // create resource
	}

	for _, pattern := range patterns {
		serveMux.Handle(pattern, handler)
	}

	return serveMux
}

// ResourceHandlerMux maps resource types to their corresponding handlers.
// It implements [http.Handler] and routes requests based on the resource type
// extracted from the request context.
type ResourceHandlerMux map[string]ResourceHandler

// ServeHTTP routes HTTP requests to the appropriate resource handler based on the resource type.
// It returns a 404 response if no handler is registered for the requested resource type.
func (m ResourceHandlerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := FromContext(r.Context())

	if request.ResourceType == "" {
		write404(w)
		return
	}

	if handler, ok := m[request.ResourceType]; ok {
		handler.ServeHTTP(w, r)
		return
	}

	write404(w)
}

// ResourceHandler contains HTTP handlers for all standard JSON:API resource operations.
// Each field represents a different operation type and can be nil if that operation is not supported.
type ResourceHandler struct {
	Create   http.Handler // Handles POST /{type} - create new resource
	Retrieve http.Handler // Handles GET /{type}/{id} - get single resource
	Update   http.Handler // Handles PATCH /{type}/{id} - update resource
	Delete   http.Handler // Handles DELETE /{type}/{id} - delete resource
	List     http.Handler // Handles GET /{type} - list resource collection
	Refs     http.Handler // Handles relationship and related resource operations
}

// ServeHTTP routes HTTP requests to the appropriate handler based on the HTTP method
// and request parameters. It supports all standard JSON:API operations including
// resource CRUD operations and relationship management.
func (h ResourceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := FromContext(r.Context())

	if request.ResourceType == "" {
		write404(w)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if request.Related && request.ResourceID != "" {
			// serve get related resources
			tryServeHTTP(w, r, h.Refs)
			return
		}
		if request.ResourceID != "" {
			// serve get resource
			tryServeHTTP(w, r, h.Retrieve)
			return
		} else {
			// serve list resource collection
			tryServeHTTP(w, r, h.List)
			return
		}
	case http.MethodPost:
		if request.ResourceID != "" && request.Relationship != "" {
			// forward relationship add to refs handler
			tryServeHTTP(w, r, h.Refs)
			return
		}
		// server create resource
		if request.ResourceID == "" {
			tryServeHTTP(w, r, h.Create)
			return
		}
	case http.MethodPatch:
		if request.ResourceID != "" && request.Relationship != "" {
			// forward relationship update to refs handler
			tryServeHTTP(w, r, h.Refs)
			return
		}
		if request.ResourceID != "" {
			// serve update relationship
			tryServeHTTP(w, r, h.Update)
			return
		}
	case http.MethodDelete:
		if request.ResourceID != "" && request.Relationship != "" {
			// forward relationship remove to refs handler
			tryServeHTTP(w, r, h.Refs)
			return
		}
		if request.ResourceID != "" {
			// serve resource delete
			tryServeHTTP(w, r, h.Delete)
		}
	default:
		write404(w)
		return
	}
}

// RelationshipHandlerMux maps relationship names to their corresponding handlers.
// It implements [http.Handler] and routes requests based on the relationship name
// extracted from the request context.
type RelationshipHandlerMux map[string]RelationshipHandler

// ServeHTTP routes HTTP requests to the appropriate relationship handler based on the relationship name.
func (m RelationshipHandlerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := FromContext(r.Context())

	if request.Relationship == "" || request.ResourceID == "" {
		write404(w)
		return
	}

	if handler, ok := m[request.Relationship]; ok {
		handler.ServeHTTP(w, r)
		return
	}

	write404(w)
}

// RelationshipHandler contains HTTP handlers for JSON:API relationship operations.
// It provides separate handlers for different relationship manipulation operations.
type RelationshipHandler struct {
	Add    http.Handler // Handles POST - add to many-to-many relationship
	Del    http.Handler // Handles DELETE - remove from many-to-many relationship
	Get    http.Handler // Handles GET - retrieve relationship data
	Update http.Handler // Handles PATCH - replace relationship data
}

// ServeHTTP routes HTTP requests to the appropriate relationship handler based on the HTTP method.
// It supports all standard JSON:API relationship operations including retrieval, updates, and modifications.
func (h RelationshipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := FromContext(r.Context())

	if request.Relationship == "" || request.ResourceID == "" {
		write404(w)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// serve get related resources
		tryServeHTTP(w, r, h.Get)
		return
	case http.MethodPost:
		// serve add to many ref
		tryServeHTTP(w, r, h.Add)
		return
	case http.MethodPatch:
		// serve update ref
		tryServeHTTP(w, r, h.Update)
		return
	case http.MethodDelete:
		// serve remove from many ref
		tryServeHTTP(w, r, h.Del)
		return
	default:
		write404(w)
		return
	}
}

// Write marshals data into a JSON:API document and writes it to the HTTP response.
// It sets the appropriate Content-Type header and HTTP status code, returning the number
// of bytes written and any marshaling or writing errors.
func write(w http.ResponseWriter, status int, data any, opts ...Options) (n int, werr error) {
	options := applyOptions(opts)
	out, err := Marshal(data, fromOptionsOverride(&options))
	if err != nil {
		return 0, err
	}

	w.Header().Add("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	return w.Write(out)
}

// WriteRef marshals data into a JSON:API document and writes it to the HTTP response.
// It sets the appropriate Content-Type header and HTTP status code, returning the number
// of bytes written and any marshaling or writing errors.
func writeRef(w http.ResponseWriter, status int, name string, data RelationshipMarshaler, opts ...Options) (n int, werr error) {
	options := applyOptions(opts)
	out, err := MarshalRef(data, name, fromOptionsOverride(&options))
	if err != nil {
		return 0, err
	}

	w.Header().Add("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	return w.Write(out)
}

// WriteErrors creates a JSON:API error document from the provided errors and writes it to the response.
// Each error is converted to a JSON:API error object with the specified HTTP status code.
func writeErrors(w http.ResponseWriter, status int, errs ...error) (n int, werr error) {
	opts := make([]Options, len(errs))
	for i, err := range errs {
		opts[i] = WithError(status, err)
	}
	return write(w, status, nil, opts...)
}

// write404 writes a standard JSON:API 404 Not Found error response.
func write404(w http.ResponseWriter) (n int, err error) {
	return writeErrors(w, http.StatusNotFound, &Error{
		Status: strconv.Itoa(http.StatusNotFound),
		Title:  http.StatusText(http.StatusNotFound),
		Detail: "Resource not found",
	})
}

// tryServeHTTP attempts to serve an HTTP request with the provided handler.
// If the handler is nil, it writes a 404 Not Found response instead.
func tryServeHTTP(w http.ResponseWriter, r *http.Request, h http.Handler) {
	if h == nil {
		write404(w)
		return
	}
	h.ServeHTTP(w, r)
}
