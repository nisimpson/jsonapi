package jsonapi

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Options defines the interface for configuration options that can be applied
// to marshaling and unmarshaling operations.
type Options interface {
	// apply applies the option to the internal options struct.
	apply(*options)
}

// options holds the internal configuration state for marshaling and unmarshaling operations.
type options struct {
	topLinks        map[string]Link         // Top-level document links
	topMeta         map[string]interface{}  // Top-level document metadata
	errors          []*Error                // List of document errors
	includes        map[string]*Resource    // Map of included resources by UID
	maxIncludeDepth int                     // Maximum depth for including related resources
	validateType    bool                    // Whether to validate resource types during unmarshaling
	linkResolver    map[string]LinkResolver // Map of link resolvers by key name for generating URLs

	// Query parameter fields used by the client layer for building request URLs.
	queryInclude    []string            // include=author,tags
	queryFields     map[string][]string // fields[articles]=title,content
	querySort       []string            // sort=-created_at,title
	queryPageNumber *[2]int             // page[number]=X&page[size]=Y (from WithPageNumber)
	queryPageCursor *struct {           // page[after]=X&page[size]=Y (from WithPageCursor)
		cursor string
		size   int
	}
	queryPageParams map[string]string   // page[key]=value (from WithPageParams)
	queryFilter     map[string]string   // filter[key]=value
	queryParams     map[string][]string // raw query parameters (from WithQueryParam)
}

// fromOptionsOverride creates an [Options] function that copies all settings from the base options.
// This is used internally to preserve existing option state when creating new option configurations.
func fromOptionsOverride(base *options) Options {
	return optionsFunc(func(options *options) {
		options.includes = base.includes
		options.maxIncludeDepth = base.maxIncludeDepth
		options.topLinks = base.topLinks
		options.topMeta = base.topMeta
		options.validateType = base.validateType
		options.errors = base.errors
		options.linkResolver = base.linkResolver
	})
}

// applyOptions creates a new options struct with default values and applies the provided options.
func applyOptions(opts []Options) options {
	options := options{
		topLinks:        make(map[string]Link),
		topMeta:         make(map[string]interface{}),
		includes:        make(map[string]*Resource),
		linkResolver:    make(map[string]LinkResolver),
		queryFields:     make(map[string][]string),
		queryPageParams: make(map[string]string),
		queryFilter:     make(map[string]string),
		queryParams:     make(map[string][]string),
		maxIncludeDepth: math.MaxInt,
		validateType:    false,
	}

	for _, opt := range opts {
		opt.apply(&options)
	}

	return options
}

// optionsFunc is a function type that implements the [Options] interface.
type optionsFunc func(*options)

// apply implements the Options interface for optionsFunc.
func (fn optionsFunc) apply(opts *options) {
	fn(opts)
}

// mergeOptions merges a list of [Options] into a single Options provider.
func mergeOptions(opts ...Options) Options {
	return optionsFunc(func(o *options) {
		for _, opt := range opts {
			opt.apply(o)
		}
	})
}

// WithTopLink adds a link to the top-level links object of the JSON:API document.
func WithTopLink(name string, link Link) Options {
	return optionsFunc(func(opts *options) {
		opts.topLinks[name] = link
	})
}

// WithTopHref adds a simple href link to the top-level links object of the JSON:API document.
func WithTopHref(name, href string) Options {
	return optionsFunc(func(opts *options) {
		opts.topLinks[name] = Link{Href: href}
	})
}

// WithTopMeta adds metadata to the top-level meta object of the JSON:API document.
func WithTopMeta(key string, value interface{}) Options {
	return optionsFunc(func(opts *options) {
		opts.topMeta[key] = value
	})
}

// WithMaxIncludeDepth sets the maximum depth for including related resources in the document.
// This prevents infinite recursion when marshaling resources with circular relationships.
func WithMaxIncludeDepth(depth int) Options {
	return optionsFunc(func(opts *options) {
		opts.maxIncludeDepth = depth
	})
}

// WithTypeValidation enables resource type validation during unmarshaling operations.
// When enabled, the unmarshaler will verify that the resource type in the document
// matches the expected type of the target struct.
func WithTypeValidation() Options {
	return optionsFunc(func(opts *options) {
		opts.validateType = true
	})
}

// WithError adds an error to the JSON:API document's error list.
// If the provided error is not already a JSON:API [Error] type, it will be converted
// to one using the provided HTTP status code. The error's title will be set to the
// standard HTTP status text, and the detail will be set to the error's message.
func WithError(status int, err error) Options {
	return optionsFunc(func(opts *options) {
		var jsonErr *Error
		if !errors.As(err, &jsonErr) {
			jsonErr = &Error{
				Status: strconv.Itoa(status),
				Title:  http.StatusText(status),
				Detail: err.Error(),
			}
		}
		opts.errors = append(opts.errors, jsonErr)
	})
}

// LinkResolver defines the interface for resolving resource and relationship links
// during marshaling operations. This allows the marshaler to generate URLs without
// requiring resources to have server awareness.
//
// The resolver receives the link key (e.g., "self", "related") and the resource
// or relationship context, then returns the appropriate Link object.
//
// Example implementation:
//
//	type MyLinkResolver struct {
//		BaseURL string
//	}
//
//	func (r MyLinkResolver) ResolveResourceLink(key string, id ResourceIdentifier) (Link, bool) {
//		if key == "self" {
//			return Link{Href: fmt.Sprintf("%s/%s/%s", r.BaseURL, id.ResourceType(), id.ResourceID())}, true
//		}
//		return Link{}, false
//	}
//
//	func (r MyLinkResolver) ResolveRelationshipLink(key string, name string, id RelationshipMarshaler) (Link, bool) {
//		base := fmt.Sprintf("%s/%s/%s", r.BaseURL, id.ResourceType(), id.ResourceID())
//		switch key {
//		case "self":
//			return Link{Href: fmt.Sprintf("%s/relationships/%s", base, name)}, true
//		case "related":
//			return Link{Href: fmt.Sprintf("%s/%s", base, name)}, true
//		}
//		return Link{}, false
//	}
type LinkResolver interface {
	// ResolveResourceLink resolves a link for a resource object.
	// The key parameter specifies the link name (e.g., "self", "edit").
	// The id parameter provides access to the resource's type and ID.
	// Returns the Link and true if the key is recognized, or zero Link and false if not.
	ResolveResourceLink(key string, id ResourceIdentifier) (Link, bool)

	// ResolveRelationshipLink resolves a link for a relationship object.
	// The key parameter specifies the link name (e.g., "self", "related").
	// The name parameter is the relationship name (e.g., "author", "tags").
	// The id parameter provides access to the parent resource's context.
	// Returns the Link and true if the key is recognized, or zero Link and false if not.
	ResolveRelationshipLink(key string, name string, id RelationshipMarshaler) (Link, bool)
}

// WithLinkResolver adds a [LinkResolver] for the specified link key during marshaling.
// The resolver will be called to generate links for resources and relationships,
// adding to any links already provided by [LinksMarshaler] implementations.
//
// Multiple resolvers can be added for different link types:
//
//	Marshal(article,
//		WithLinkResolver("self", selfResolver),
//		WithLinkResolver("edit", editResolver),
//	)
//
// The key parameter determines which link name the resolver handles.
// Common keys include "self", "related", "edit", "delete", etc.
func WithLinkResolver(key string, resolver LinkResolver) Options {
	return optionsFunc(func(opts *options) {
		opts.linkResolver[key] = resolver
	})
}

// SelfLinkResolver is a default implementation of [LinkResolver] that generates
// standard JSON:API links using configurable URL patterns.
//
// It supports generating "self" links for resources and both "self" and "related"
// links for relationships using printf-style format strings.
//
// Example usage:
//
//	resolver := SelfLinkResolver{
//		BaseURL:                    "https://api.example.com",
//		SelfResourcePattern:        "%s/%s/%s",                  // https://api.example.com/articles/123
//		SelfRelationshipPattern:    "%s/%s/%s/relationships/%s", // https://api.example.com/articles/123/relationships/author
//		RelatedRelationshipPattern: "%s/%s/%s/%s",               // https://api.example.com/articles/123/author
//	}
type SelfLinkResolver struct {
	BaseURL                    string // Base URL for all generated links
	SelfResourcePattern        string // Format pattern for resource self links: {base}/{type}/{id}
	SelfRelationshipPattern    string // Format pattern for relationship self links: {base}/{type}/{id}/relationships/{name}
	RelatedRelationshipPattern string // Format pattern for relationship related links: {base}/{type}/{id}/{name}
}

// ResolveResourceLink generates a "self" link for the given resource.
// Returns the link and true if the key is "self", otherwise returns zero [Link] and false.
func (d SelfLinkResolver) ResolveResourceLink(key string, id ResourceIdentifier) (Link, bool) {
	if key == "self" {
		href := fmt.Sprintf(d.SelfResourcePattern, d.BaseURL, id.ResourceType(), id.ResourceID())
		return Link{Href: href}, true
	}
	return Link{}, false
}

// ResolveRelationshipLink generates "self" and "related" links for relationships.
// Returns the appropriate link and true if the key is recognized ("self" or "related"),
// otherwise returns zero [Link] and false.
func (d SelfLinkResolver) ResolveRelationshipLink(key string, name string, id RelationshipMarshaler) (Link, bool) {
	if key == "self" {
		href := fmt.Sprintf(d.SelfRelationshipPattern, d.BaseURL, id.ResourceType(), id.ResourceID(), name)
		return Link{Href: href}, true
	}
	if key == "related" {
		href := fmt.Sprintf(d.RelatedRelationshipPattern, d.BaseURL, id.ResourceType(), id.ResourceID(), name)
		return Link{Href: href}, true
	}
	return Link{}, false
}

// WithDefaultLinks creates a convenient option that adds standard JSON:API links
// using the [SelfLinkResolver] with default URL patterns.
//
// This is equivalent to manually creating a SelfLinkResolver and adding it for
// both "self" and "related" link keys.
//
// The generated URL patterns follow JSON:API conventions:
//   - Resource self links: {baseURL}/{type}/{id}
//   - Relationship self links: {baseURL}/{type}/{id}/relationships/{name}
//   - Relationship related links: {baseURL}/{type}/{id}/{name}
//
// Example usage:
//
//	Marshal(article, WithDefaultLinks("https://api.example.com"))
//
// This will generate links like:
//   - https://api.example.com/articles/123 (resource self)
//   - https://api.example.com/articles/123/relationships/author (relationship self)
//   - https://api.example.com/articles/123/author (relationship related)
func WithDefaultLinks(baseURL string) Options {
	resolver := SelfLinkResolver{
		BaseURL:                    baseURL,
		SelfResourcePattern:        "%s/%s/%s",                  // {base}/{type}/{id}
		SelfRelationshipPattern:    "%s/%s/%s/relationships/%s", // {base}/{type}/{id}/relationships/{name}
		RelatedRelationshipPattern: "%s/%s/%s/%s",               // {base}/{type}/{id}/{name}
	}
	return mergeOptions(
		WithLinkResolver("self", resolver),
		WithLinkResolver("related", resolver),
	)
}

// WithInclude specifies related resources to include in the response.
// The relationships are joined as a comma-separated list in the `include` query parameter.
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithInclude("author", "tags"))
//	// produces: ?include=author,tags
func WithInclude(relationships ...string) Options {
	return optionsFunc(func(opts *options) {
		opts.queryInclude = append(opts.queryInclude, relationships...)
	})
}

// WithFields specifies sparse fieldsets for a resource type.
// The fields are joined as a comma-separated list in the `fields[{type}]` query parameter.
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithFields("articles", "title", "content"))
//	// produces: ?fields[articles]=title,content
func WithFields(resourceType string, fields ...string) Options {
	return optionsFunc(func(opts *options) {
		opts.queryFields[resourceType] = append(opts.queryFields[resourceType], fields...)
	})
}

// WithSort specifies sort fields for the request.
// Prefix a field with "-" for descending order.
// The fields are joined as a comma-separated list in the `sort` query parameter.
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithSort("-created_at", "title"))
//	// produces: ?sort=-created_at,title
func WithSort(fields ...string) Options {
	return optionsFunc(func(opts *options) {
		opts.querySort = append(opts.querySort, fields...)
	})
}

// WithPageNumber specifies offset/number-based pagination.
// Produces `page[number]=X&page[size]=Y` in the request URL query string.
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithPageNumber(1, 25))
//	// produces: ?page[number]=1&page[size]=25
func WithPageNumber(number, size int) Options {
	return optionsFunc(func(opts *options) {
		opts.queryPageNumber = &[2]int{number, size}
	})
}

// WithPageCursor specifies cursor-based pagination.
// Produces `page[after]=X&page[size]=Y` in the request URL query string.
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithPageCursor("abc123", 25))
//	// produces: ?page[after]=abc123&page[size]=25
func WithPageCursor(cursor string, size int) Options {
	return optionsFunc(func(opts *options) {
		opts.queryPageCursor = &struct {
			cursor string
			size   int
		}{cursor: cursor, size: size}
	})
}

// WithPageParams specifies generic page parameters as key-value pairs.
// Produces `page[key]=value` for each entry in the request URL query string.
// Use as an escape hatch for custom pagination strategies not covered by
// [WithPageNumber] or [WithPageCursor].
//
// Example:
//
//	client.List(ctx, "articles", jsonapi.WithPageParams(map[string]string{"offset": "10", "limit": "25"}))
//	// produces: ?page[offset]=10&page[limit]=25
func WithPageParams(params map[string]string) Options {
	return optionsFunc(func(opts *options) {
		for k, v := range params {
			opts.queryPageParams[k] = v
		}
	})
}

// WithFilter specifies a filter parameter as a key-value pair.
// Produces `filter[key]=value` in the request URL query string.
//
// Example:
//
//	client.List(ctx, "articles",
//	    jsonapi.WithFilter("status", "published"),
//	    jsonapi.WithFilter("author", "john"),
//	)
//	// produces: ?filter[status]=published&filter[author]=john
func WithFilter(key, value string) Options {
	return optionsFunc(func(opts *options) {
		opts.queryFilter[key] = value
	})
}

// WithQueryParam adds a raw query parameter to the request URL.
// The key and value are used as-is without any wrapping or formatting.
// Multiple calls with the same key will append values.
//
// Use this for non-standard or vendor-specific query parameters that don't
// fit the JSON:API conventions covered by WithInclude, WithFields, etc.
//
// Example:
//
//	client.List(ctx, "articles",
//	    jsonapi.WithQueryParam("search", "golang"),
//	    jsonapi.WithQueryParam("version", "2"),
//	)
//	// produces: ?search=golang&version=2
func WithQueryParam(key, value string) Options {
	return optionsFunc(func(opts *options) {
		opts.queryParams[key] = append(opts.queryParams[key], value)
	})
}

// buildQueryParams encodes all query parameter fields into url.Values.
// It produces JSON:API compliant query parameters:
//   - include=author,tags (comma-separated)
//   - fields[type]=field1,field2 (comma-separated per type)
//   - sort=-created_at,title (comma-separated)
//   - page[number]=X&page[size]=Y (from WithPageNumber)
//   - page[after]=X&page[size]=Y (from WithPageCursor)
//   - page[key]=value (from WithPageParams)
//   - filter[key]=value (from WithFilter)
//   - key=value (from WithQueryParam)
func (o *options) buildQueryParams() url.Values {
	params := url.Values{}

	// Encode include as comma-separated list.
	if len(o.queryInclude) > 0 {
		params.Set("include", strings.Join(o.queryInclude, ","))
	}

	// Encode fields[type] as comma-separated list per type.
	// Sort map keys for deterministic output.
	if len(o.queryFields) > 0 {
		keys := make([]string, 0, len(o.queryFields))
		for k := range o.queryFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			params.Set(fmt.Sprintf("fields[%s]", k), strings.Join(o.queryFields[k], ","))
		}
	}

	// Encode sort as comma-separated list.
	if len(o.querySort) > 0 {
		params.Set("sort", strings.Join(o.querySort, ","))
	}

	// Encode page[number] and page[size] from WithPageNumber.
	if o.queryPageNumber != nil {
		params.Set("page[number]", strconv.Itoa(o.queryPageNumber[0]))
		params.Set("page[size]", strconv.Itoa(o.queryPageNumber[1]))
	}

	// Encode page[after] and page[size] from WithPageCursor.
	if o.queryPageCursor != nil {
		params.Set("page[after]", o.queryPageCursor.cursor)
		params.Set("page[size]", strconv.Itoa(o.queryPageCursor.size))
	}

	// Encode page[key]=value from WithPageParams.
	// Sort map keys for deterministic output.
	if len(o.queryPageParams) > 0 {
		keys := make([]string, 0, len(o.queryPageParams))
		for k := range o.queryPageParams {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			params.Set(fmt.Sprintf("page[%s]", k), o.queryPageParams[k])
		}
	}

	// Encode filter[key]=value from WithFilter.
	// Sort map keys for deterministic output.
	if len(o.queryFilter) > 0 {
		keys := make([]string, 0, len(o.queryFilter))
		for k := range o.queryFilter {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			params.Set(fmt.Sprintf("filter[%s]", k), o.queryFilter[k])
		}
	}

	// Encode raw query parameters from WithQueryParam.
	// Sort map keys for deterministic output.
	if len(o.queryParams) > 0 {
		keys := make([]string, 0, len(o.queryParams))
		for k := range o.queryParams {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range o.queryParams[k] {
				params.Add(k, v)
			}
		}
	}

	return params
}
