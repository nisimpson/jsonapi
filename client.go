package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// URLResolver controls how the client constructs request URLs for each
// JSON:API operation type. This mirrors the server-side RequestResolver
// pattern but for outbound client requests.
type URLResolver interface {
	// ResolveResourceURL returns the URL for a single resource (GET/PATCH/DELETE /{type}/{id}).
	ResolveResourceURL(resourceType, id string) string
	// ResolveCollectionURL returns the URL for a resource collection (GET/POST /{type}).
	ResolveCollectionURL(resourceType string) string
	// ResolveRelationshipURL returns the URL for a relationship endpoint
	// (GET/PATCH/POST/DELETE /{type}/{id}/relationships/{ref}).
	ResolveRelationshipURL(resourceType, id, relationship string) string
	// ResolveRelatedURL returns the URL for a related resource endpoint
	// (GET /{type}/{id}/{related}).
	ResolveRelatedURL(resourceType, id, relationship string) string
}

// DefaultURLResolver implements URLResolver using standard JSON:API URL conventions.
type DefaultURLResolver struct {
	BaseURL string
}

// ResolveResourceURL returns the URL for a single resource: {baseURL}/{type}/{id}.
func (d DefaultURLResolver) ResolveResourceURL(resourceType, id string) string {
	return d.BaseURL + "/" + resourceType + "/" + id
}

// ResolveCollectionURL returns the URL for a resource collection: {baseURL}/{type}.
func (d DefaultURLResolver) ResolveCollectionURL(resourceType string) string {
	return d.BaseURL + "/" + resourceType
}

// ResolveRelationshipURL returns the URL for a relationship endpoint:
// {baseURL}/{type}/{id}/relationships/{relationship}.
func (d DefaultURLResolver) ResolveRelationshipURL(resourceType, id, relationship string) string {
	return d.BaseURL + "/" + resourceType + "/" + id + "/relationships/" + relationship
}

// ResolveRelatedURL returns the URL for a related resource endpoint:
// {baseURL}/{type}/{id}/{relationship}.
func (d DefaultURLResolver) ResolveRelatedURL(resourceType, id, relationship string) string {
	return d.BaseURL + "/" + resourceType + "/" + id + "/" + relationship
}

// RequestMiddleware modifies an outgoing HTTP request before it is sent.
type RequestMiddleware func(r *http.Request) (*http.Request, error)

// ResponseMiddleware modifies an incoming HTTP response after it is received.
type ResponseMiddleware func(r *http.Response) (*http.Response, error)

// Client is a JSON:API HTTP client that wraps net/http.Client and provides
// typed methods for all standard JSON:API operations.
type Client struct {
	httpClient         *http.Client
	resolver           URLResolver
	requestMiddleware  []RequestMiddleware
	responseMiddleware []ResponseMiddleware
}

// ClientOption configures a Client.
type ClientOption interface {
	applyClient(*Client)
}

// clientOptionFunc is a function type that implements the ClientOption interface.
type clientOptionFunc func(*Client)

func (f clientOptionFunc) applyClient(c *Client) { f(c) }

// NewClient creates a new JSON:API client with the given base URL and options.
// It defaults to http.DefaultClient and DefaultURLResolver when no custom
// implementations are provided.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
		resolver:   DefaultURLResolver{BaseURL: baseURL},
	}
	for _, opt := range opts {
		opt.applyClient(c)
	}
	return c
}

// WithHTTPClient sets a custom net/http.Client for the JSON:API client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.httpClient = hc
	})
}

// WithURLResolver sets a custom URLResolver for the JSON:API client.
func WithURLResolver(r URLResolver) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.resolver = r
	})
}

// WithRequestMiddleware adds request middleware to the client. Middleware
// is applied in the order it was registered before each request is sent.
func WithRequestMiddleware(m ...RequestMiddleware) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.requestMiddleware = append(c.requestMiddleware, m...)
	})
}

// WithResponseMiddleware adds response middleware to the client. Middleware
// is applied in the order it was registered after each response is received.
func WithResponseMiddleware(m ...ResponseMiddleware) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.responseMiddleware = append(c.responseMiddleware, m...)
	})
}

// Response wraps an HTTP response from a JSON:API server and provides
// convenience methods for accessing the response document.
type Response struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Header contains the response headers.
	Header http.Header
	// document is the parsed JSON:API document (nil for 204 No Content).
	document *Document
}

// Unmarshal unmarshals the primary data into the target.
// For single resources, target should be a pointer to a struct implementing ResourceUnmarshaler.
// For collections, target should be a pointer to a slice of structs implementing ResourceUnmarshaler.
// Returns io.EOF when the document is nil (e.g., 204 No Content responses).
func (r *Response) Unmarshal(target interface{}, opts ...Options) error {
	if r.document == nil {
		return fmt.Errorf("%w: no document to unmarshal", io.EOF)
	}
	return r.document.UnmarshalData(target, opts...)
}

// UnmarshalRef unmarshals relationship data from the response document into the target.
// The name parameter specifies which relationship to extract. The target must implement
// RelationshipUnmarshaler. Returns io.EOF when the document is nil.
func (r *Response) UnmarshalRef(name string, target RelationshipUnmarshaler, opts ...Options) error {
	if r.document == nil {
		return fmt.Errorf("%w: no document to unmarshal", io.EOF)
	}

	options := applyOptions(opts)

	var (
		relationships   = target.Relationships()
		relType, exists = relationships[name]
	)

	if !exists {
		return fmt.Errorf("relationship %q not found for resource %q", name, target.ResourceType())
	}

	if r.document.Data == nil {
		if relType == RelationToMany {
			return fmt.Errorf("null data not allowed for to-many relationship %q: use empty array instead", name)
		}
		return target.UnmarshalRef(name, "", nil)
	}

	relData := &RelationshipData{isMany: r.document.Data.isMany}

	if r.document.Data.isMany {
		relData.many = make([]Ref, len(r.document.Data.many))
		for i, res := range r.document.Data.many {
			relData.many[i] = Ref{ID: res.ID, Type: res.Type}
		}
	} else {
		relData.one = Ref{ID: r.document.Data.one.ID, Type: r.document.Data.one.Type}
	}

	rel := &Relationship{Data: relData}
	return unmarshalRelationship(rel, name, target, &options)
}

// UnmarshalIncluded unmarshals included resources of the specified type into the target slice.
// The target must be a pointer to a slice of structs implementing ResourceUnmarshaler.
// When no included resources match the specified type, the target is set to an empty slice
// and no error is returned.
func (r *Response) UnmarshalIncluded(resourceType string, target interface{}, opts ...Options) error {
	if r.document == nil {
		return fmt.Errorf("%w: no document to unmarshal", io.EOF)
	}

	if target == nil {
		return fmt.Errorf("target must not be nil")
	}

	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a ptr")
	}

	targetSlice := targetVal.Elem()
	if targetSlice.Kind() != reflect.Slice {
		return fmt.Errorf("target must be a pointer to a slice")
	}

	options := applyOptions(opts)
	elemType := targetSlice.Type().Elem()

	// Start with an empty slice of the correct type.
	result := reflect.MakeSlice(targetSlice.Type(), 0, 0)

	for _, res := range r.document.Included {
		if res.Type != resourceType {
			continue
		}
		elem := reflect.New(elemType)
		if err := unmarshalOne(*res, elem.Interface(), &options); err != nil {
			return fmt.Errorf("unmarshal included resource: %w", err)
		}
		result = reflect.Append(result, elem.Elem())
	}

	targetVal.Elem().Set(result)
	return nil
}

// Document returns the full parsed JSON:API document from the response.
// Returns nil for responses without a body (e.g., 204 No Content).
func (r *Response) Document() *Document {
	return r.document
}

// Links returns the top-level links from the response document.
// Returns nil when the document is nil.
func (r *Response) Links() map[string]Link {
	if r.document == nil {
		return nil
	}
	return r.document.Links
}

// Meta returns the top-level meta from the response document.
// Returns nil when the document is nil.
func (r *Response) Meta() map[string]interface{} {
	if r.document == nil {
		return nil
	}
	return r.document.Meta
}

// Errors returns the error objects from the response document.
// Returns nil when the document is nil.
func (r *Response) Errors() []*Error {
	if r.document == nil {
		return nil
	}
	return r.document.Errors
}

// HasErrors returns true if the response contains JSON:API errors.
func (r *Response) HasErrors() bool {
	return len(r.Errors()) > 0
}

// ResponseError is returned when the server responds with a non-2xx status code.
// It wraps the full Response so callers can inspect the status code, headers, and
// JSON:API error objects using errors.As:
//
//	resp, err := client.Create(ctx, article)
//	if err != nil {
//	    var respErr *jsonapi.ResponseError
//	    if errors.As(err, &respErr) {
//	        fmt.Println(respErr.StatusCode)
//	        for _, e := range respErr.Errors() {
//	            fmt.Println(e.Detail)
//	        }
//	    }
//	    return err
//	}
type ResponseError struct {
	*Response
}

// Error implements the error interface. It returns a string like "HTTP 422: Unprocessable Entity".
func (e *ResponseError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, http.StatusText(e.StatusCode))
}

// Fetch retrieves a single resource by type and ID.
// It sends an HTTP GET request to the URL resolved by the configured URLResolver.
func (c *Client) Fetch(ctx context.Context, resourceType, id string, opts ...Options) (*Response, error) {
	url := c.resolver.ResolveResourceURL(resourceType, id)
	return c.doRequest(ctx, http.MethodGet, url, nil, opts...)
}

// List retrieves a collection of resources by type.
// It sends an HTTP GET request to the URL resolved by the configured URLResolver.
func (c *Client) List(ctx context.Context, resourceType string, opts ...Options) (*Response, error) {
	url := c.resolver.ResolveCollectionURL(resourceType)
	return c.doRequest(ctx, http.MethodGet, url, nil, opts...)
}

// Create creates a new resource on the server.
// It marshals the resource into a JSON:API document and sends an HTTP POST request
// to the collection URL resolved by the configured URLResolver.
func (c *Client) Create(ctx context.Context, resource ResourceIdentifier, opts ...Options) (*Response, error) {
	body, err := Marshal(resource, opts...)
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}
	url := c.resolver.ResolveCollectionURL(resource.ResourceType())
	return c.doRequest(ctx, http.MethodPost, url, body, opts...)
}

// Update updates an existing resource on the server.
// It marshals the resource into a JSON:API document and sends an HTTP PATCH request
// to the resource URL resolved by the configured URLResolver.
func (c *Client) Update(ctx context.Context, resource ResourceIdentifier, opts ...Options) (*Response, error) {
	body, err := Marshal(resource, opts...)
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}
	url := c.resolver.ResolveResourceURL(resource.ResourceType(), resource.ResourceID())
	return c.doRequest(ctx, http.MethodPatch, url, body, opts...)
}

// Delete removes a resource from the server.
// It sends an HTTP DELETE request to the URL resolved by the configured URLResolver.
func (c *Client) Delete(ctx context.Context, resourceType, id string, opts ...Options) (*Response, error) {
	url := c.resolver.ResolveResourceURL(resourceType, id)
	return c.doRequest(ctx, http.MethodDelete, url, nil, opts...)
}

// FetchRef retrieves relationship data for a resource.
// It sends an HTTP GET request to the relationship URL resolved by the configured URLResolver.
func (c *Client) FetchRef(ctx context.Context, resourceType, id, relationship string, opts ...Options) (*Response, error) {
	url := c.resolver.ResolveRelationshipURL(resourceType, id, relationship)
	return c.doRequest(ctx, http.MethodGet, url, nil, opts...)
}

// UpdateRef replaces relationship data for a resource.
// It marshals the relationship data using MarshalRef and sends an HTTP PATCH request
// to the relationship URL resolved by the configured URLResolver.
func (c *Client) UpdateRef(ctx context.Context, resource RelationshipMarshaler, relationship string, opts ...Options) (*Response, error) {
	body, err := MarshalRef(resource, relationship, opts...)
	if err != nil {
		return nil, fmt.Errorf("marshal relationship: %w", err)
	}
	url := c.resolver.ResolveRelationshipURL(resource.ResourceType(), resource.ResourceID(), relationship)
	return c.doRequest(ctx, http.MethodPatch, url, body, opts...)
}

// AddRef adds members to a to-many relationship.
// It marshals the relationship data using MarshalRef and sends an HTTP POST request
// to the relationship URL resolved by the configured URLResolver.
func (c *Client) AddRef(ctx context.Context, resource RelationshipMarshaler, relationship string, opts ...Options) (*Response, error) {
	body, err := MarshalRef(resource, relationship, opts...)
	if err != nil {
		return nil, fmt.Errorf("marshal relationship: %w", err)
	}
	url := c.resolver.ResolveRelationshipURL(resource.ResourceType(), resource.ResourceID(), relationship)
	return c.doRequest(ctx, http.MethodPost, url, body, opts...)
}

// RemoveRef removes members from a to-many relationship.
// It marshals the relationship data using MarshalRef and sends an HTTP DELETE request
// to the relationship URL resolved by the configured URLResolver.
func (c *Client) RemoveRef(ctx context.Context, resource RelationshipMarshaler, relationship string, opts ...Options) (*Response, error) {
	body, err := MarshalRef(resource, relationship, opts...)
	if err != nil {
		return nil, fmt.Errorf("marshal relationship: %w", err)
	}
	url := c.resolver.ResolveRelationshipURL(resource.ResourceType(), resource.ResourceID(), relationship)
	return c.doRequest(ctx, http.MethodDelete, url, body, opts...)
}

// FetchRelated retrieves the full related resources for a relationship.
// It sends an HTTP GET request to the related resource URL resolved by the configured URLResolver.
func (c *Client) FetchRelated(ctx context.Context, resourceType, id, relationship string, opts ...Options) (*Response, error) {
	url := c.resolver.ResolveRelatedURL(resourceType, id, relationship)
	return c.doRequest(ctx, http.MethodGet, url, nil, opts...)
}

// PageIterator traverses pages of a JSON:API resource collection by following
// pagination links from response documents. It follows the sql.Rows / bufio.Scanner
// pattern:
//
//	iter := client.Pages(resp)
//	for iter.Next(ctx) {
//	    var items []MyResource
//	    iter.Items(&items)
//	    // process items...
//	}
//	if err := iter.Err(); err != nil {
//	    // handle error
//	}
type PageIterator struct {
	client   *Client
	response *Response
	err      error
}

// Pages returns a PageIterator for the given response. The iterator follows
// "next" pagination links automatically. The provided response is treated as
// the first page; calling Next fetches subsequent pages.
func (c *Client) Pages(response *Response) *PageIterator {
	return &PageIterator{
		client:   c,
		response: response,
	}
}

// Next fetches the next page by following the "next" link in the current
// response's Links. It returns false when no more pages are available or
// when an error occurs. The caller should check Err() after Next returns false.
func (p *PageIterator) Next(ctx context.Context) bool {
	if p.response == nil {
		return false
	}

	links := p.response.Links()
	if links == nil {
		return false
	}

	next, ok := links["next"]
	if !ok || next.Href == "" {
		return false
	}

	resp, err := p.client.doRequest(ctx, http.MethodGet, next.Href, nil)
	if err != nil {
		p.err = err
		return false
	}

	p.response = resp
	return true
}

// Items unmarshals the current page's primary data into the target.
// It delegates to the current response's Unmarshal method.
func (p *PageIterator) Items(target interface{}, opts ...Options) error {
	if p.response == nil {
		return fmt.Errorf("%w: no response available", io.EOF)
	}
	return p.response.Unmarshal(target, opts...)
}

// Response returns the Response for the current page.
func (p *PageIterator) Response() *Response {
	return p.response
}

// Err returns any error encountered during iteration. It returns nil when
// iteration completed normally (no more "next" links).
func (p *PageIterator) Err() error {
	return p.err
}

// jsonapiContentType is the JSON:API media type used for Accept and Content-Type headers.
const jsonapiContentType = "application/vnd.api+json"

// doRequest builds and sends an HTTP request, applying middleware and parsing the response.
// It handles JSON:API headers, query parameters, middleware chains, and response parsing
// including 204 No Content, error responses, and successful responses.
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte, opts ...Options) (*Response, error) {
	// Build query parameters from options and append to URL.
	o := applyOptions(opts)
	qp := o.buildQueryParams()
	if encoded := qp.Encode(); encoded != "" {
		if strings.Contains(url, "?") {
			url += "&" + encoded
		} else {
			url += "?" + encoded
		}
	}

	// Build the HTTP request with context and optional body.
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set JSON:API headers.
	req.Header.Set("Accept", jsonapiContentType)
	if body != nil {
		req.Header.Set("Content-Type", jsonapiContentType)
	}

	// Apply request middleware in registration order.
	for _, mw := range c.requestMiddleware {
		req, err = mw(req)
		if err != nil {
			return nil, fmt.Errorf("request middleware: %w", err)
		}
	}

	// Send the request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Apply response middleware in registration order.
	for _, mw := range c.responseMiddleware {
		resp, err = mw(resp)
		if err != nil {
			return nil, fmt.Errorf("response middleware: %w", err)
		}
	}

	// Handle 204 No Content — return a Response with nil document.
	if resp.StatusCode == http.StatusNoContent {
		return &Response{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			document:   nil,
		}, nil
	}

	// Read the response body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// For non-2xx responses, always return a ResponseError.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r := &Response{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
		}
		doc := &Document{}
		if err := json.Unmarshal(respBody, doc); err == nil {
			r.document = doc
		}
		return nil, &ResponseError{Response: r}
	}

	// Parse successful response body into a Document.
	doc := &Document{}
	if err := json.Unmarshal(respBody, doc); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		document:   doc,
	}, nil
}
