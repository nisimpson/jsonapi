package jsonapi

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
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
