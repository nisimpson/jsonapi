// Package server provides HTTP server utilities for building JSON:API compliant web services.
// It includes request context management, resource handlers, and routing utilities that
// simplify the creation of JSON:API endpoints following the specification.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/nisimpson/jsonapi"
)

// DefaultHandler creates a default HTTP handler with standard JSON:API routes
// configured. It sets up all the conventional JSON:API endpoints including
// resource CRUD operations and relationship management endpoints, using the
// provided ResourceHandlerMux for routing.
//
// The following endpoints are configured:
//
//	"GET    /{type}"                                   // Search/list resources of a type
//	"GET    /{type}/{id}"                              // Get a single resource by ID
//	"POST   /{type}"                                   // Create a new resource
//	"PATCH  /{type}/{id}"                              // Update an existing resource
//	"DELETE /{type}/{id}"                              // Delete a resource
//	"GET    /{type}/{id}/relationships/{relationship}" // Get a resource's relationship
//	"GET    /{type}/{id}/{related}"                    // Get related resources
//	"POST   /{type}/{id}/relationships/{relationship}" // Add to a to-many relationship
//	"PATCH  /{type}/{id}/relationships/{relationship}" // Update a relationship
//	"DELETE /{type}/{id}/relationships/{relationship}" // Remove from a to-many relationship
func DefaultHandler(mux ResourceHandlerMux) http.Handler {
	var (
		handleWithContext = UseRequestContextResolver(mux, DefaultContextResolver{})
		handler           = UseContentNegotiation(handleWithContext)
		serveMux          = http.NewServeMux()
	)

	serveMux.Handle("GET /{type}", handler)                                      // Search resource collection
	serveMux.Handle("GET /{type}/{id}", handler)                                 // Get resource
	serveMux.Handle("POST /{type}", handler)                                     // Create resource
	serveMux.Handle("PATCH /{type}/{id}", handler)                               // Update resource
	serveMux.Handle("DELETE /{type}/{id}", handler)                              // Delete resource
	serveMux.Handle("GET /{type}/{id}/relationships/{relationship}", handler)    // Get resource relationship
	serveMux.Handle("GET /{type}/{id}/{related}", handler)                       // Get related resources
	serveMux.Handle("POST /{type}/{id}/relationships/{relationship}", handler)   // Add to resource to-many relationship
	serveMux.Handle("PATCH /{type}/{id}/relationships/{relationship}", handler)  // Update resource relationship
	serveMux.Handle("DELETE /{type}/{id}/relationships/{relationship}", handler) // Remove from resource to-many relationship

	return serveMux
}

// ResourceHandler provides HTTP handlers for different JSON:API resource operations.
// It routes requests to appropriate handlers based on the HTTP method and request
// context, following JSON:API conventions for resource manipulation.
type ResourceHandler struct {
	Get          http.Handler // Handler for GET requests to retrieve a single resource
	Create       http.Handler // Handler for POST requests to create new resources
	Update       http.Handler // Handler for PATCH requests to update existing resources
	Delete       http.Handler // Handler for DELETE requests to remove resources
	Search       http.Handler // Handler for GET requests to search/list resources
	Relationship http.Handler // Handler for relationship-specific operations
}

// ServeHTTP implements the [http.Handler] interface for [ResourceHandler].
// It routes incoming requests to the appropriate handler based on the HTTP method
// and request context information, following JSON:API routing conventions.
func (rh ResourceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestContext, ok := GetRequestContext(r.Context())
	if !ok {
		http.Error(w, "Request context not found", http.StatusInternalServerError)
		return
	}

	if requestContext.ResourceType == "" {
		http.Error(w, "Request context missing resource type", http.StatusInternalServerError)
		return
	}

	if requestContext.Relationship != "" {
		rh.Relationship.ServeHTTP(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if requestContext.ResourceID != "" && rh.Get != nil {
			rh.Get.ServeHTTP(w, r)
			return
		} else if rh.Search != nil {
			rh.Search.ServeHTTP(w, r)
			return
		}
	case http.MethodPost:
		if requestContext.ResourceID == "" && rh.Create != nil {
			rh.Create.ServeHTTP(w, r)
			return
		}
	case http.MethodPatch:
		if requestContext.ResourceID != "" && rh.Update != nil {
			rh.Update.ServeHTTP(w, r)
			return
		}
	case http.MethodDelete:
		if requestContext.ResourceID != "" && rh.Delete != nil {
			rh.Delete.ServeHTTP(w, r)
			return
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.Error(w, "Resource not found", http.StatusNotFound)
}

// ResourceHandlerMux is a multiplexer that routes requests to different resource
// handlers based on the resource type. It maps resource type names to their
// corresponding HTTP handlers, enabling multi-resource JSON:API services.
type ResourceHandlerMux map[string]http.Handler

// ServeHTTP implements the [http.Handler] interface for [ResourceHandlerMux].
// It routes incoming requests to the appropriate resource handler based on
// the resource type specified in the request context.
func (rm ResourceHandlerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestContext, ok := GetRequestContext(r.Context())

	if !ok {
		http.Error(w, "Request context not found", http.StatusInternalServerError)
		return
	}

	if requestContext.ResourceType == "" {
		http.Error(w, "Request context missing resource type", http.StatusInternalServerError)
		return
	}

	resourceHandler, ok := rm[requestContext.ResourceType]
	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	resourceHandler.ServeHTTP(w, r)
}

// RelationshipHandler provides HTTP handlers for JSON:API relationship operations.
// It supports the full range of relationship manipulation operations as defined
// in the JSON:API specification, including fetching, adding, updating, and removing
// relationship linkages.
type RelationshipHandler struct {
	Get    http.Handler // Handler for GET requests to fetch relationship linkage
	Add    http.Handler // Handler for POST requests to add to to-many relationships
	Update http.Handler // Handler for PATCH requests to update relationship linkage
	Delete http.Handler // Handler for DELETE requests to remove from to-many relationships
}

// ServeHTTP implements the [http.Handler] interface for [RelationshipHandler].
// It routes relationship requests to the appropriate handler based on the HTTP
// method, following JSON:API relationship operation conventions.
func (rh RelationshipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestContext, ok := GetRequestContext(r.Context())
	if !ok {
		http.Error(w, "Request context not found", http.StatusInternalServerError)
		return
	}

	if requestContext.Relationship == "" {
		http.Error(w, "Request context missing relationship", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if requestContext.ResourceID != "" && rh.Get != nil {
			rh.Get.ServeHTTP(w, r)
			return
		}
	case http.MethodPost:
		if requestContext.ResourceID != "" && rh.Add != nil {
			rh.Add.ServeHTTP(w, r)
			return
		}
	case http.MethodPatch:
		if requestContext.ResourceID != "" && rh.Update != nil {
			rh.Update.ServeHTTP(w, r)
			return
		}
	case http.MethodDelete:
		if requestContext.ResourceID != "" && rh.Delete != nil {
			rh.Delete.ServeHTTP(w, r)
			return
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If we get here, the relationship was not found
	http.Error(w, "Resource not found", http.StatusNotFound)
}

// Response represents a JSON:API HTTP response with status code, headers, and
// a JSON:API document body. In conjunction with [HandlerFunc], Response provides
// a structured way to return JSON:API compliant responses from handler functions.
type Response struct {
	Status int               // HTTP status code for the response
	Header http.Header       // HTTP headers to include in the response
	Body   *jsonapi.Document // JSON:API document to send as the response body
}

// HandlerFunc is a function type that processes JSON:API requests and returns
// structured responses. It receives the parsed request context and HTTP request,
// and returns a [Response] struct. This provides a more convenient
// way to write JSON:API handlers compared to the standard [http.HandlerFunc] type.
type HandlerFunc func(*RequestContext, *http.Request) (res Response)

// ServeHTTP implements the http.Handler interface for Handler functions.
// It calls the handler function with the request context and handles the
// response formatting, including setting appropriate headers and encoding
// the JSON:API document body.
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestContext, ok := GetRequestContext(r.Context())
	if !ok {
		Error(w, errors.New("request context not found"), http.StatusInternalServerError)
		return
	}

	res := h(requestContext, r)

	for k, v := range res.Header {
		w.Header()[k] = v
	}

	Write(w, res.Body, res.Status)
}

// Write sends a JSON:API document response to the client with the specified HTTP status code.
// The response will have Content-Type set to application/vnd.api+json and include the JSON-encoded document body.
// If the body is nil, only the status code will be written without a response body.
// The function handles setting the appropriate Content-Type header and encoding the document as JSON.
func Write(w http.ResponseWriter, body *jsonapi.Document, status int) {
	if body == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// Error writes a JSON:API error response to the provided [http.ResponseWriter].
// It accepts multiple types of error inputs and formats them according to
// the JSON:API specification for error objects.
//
// Supported error types:
//   - jsonapi.Error: Used directly
//   - jsonapi.MultiError: All errors are included in the response
//   - Other error types: Converted to [jsonapi.Error] with the provided status
//
// The response will have Content-Type set to application/vnd.api+json and include
// a JSON:API document containing the formatted error(s).
func Error(w http.ResponseWriter, err error, status int) {
	doc := jsonapi.NewErrorDocument(err)
	for i, e := range doc.Errors {
		if e.Status == "" {
			e.Status = strconv.Itoa(status)
		}
		if e.Title == "" {
			e.Title = http.StatusText(status)
		}
		doc.Errors[i] = e
	}

	Write(w, doc, status)
}
