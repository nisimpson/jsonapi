package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
)

// Struct tag constants for JSON:API field definitions.
// These constants define the tag name and tag values used in struct field tags
// to control JSON:API marshaling and unmarshaling behavior.
const (
	// StructTagName is the name of the struct tag used for JSON:API field definitions.
	// Example:
	//
	// 	`jsonapi:"primary,users"`
	StructTagName = "jsonapi"

	// TagValuePrimary indicates a field contains the primary resource ID and type.
	// Format: `jsonapi:"primary,resource-type"`
	// Example:
	//
	// 	`jsonapi:"primary,users"`
	TagValuePrimary = "primary"

	// TagValueAttribute indicates a field should be marshaled as a JSON:API attribute.
	// Format: `jsonapi:"attr,attribute-name[,omitempty]"`
	// Example:
	//
	// 	`jsonapi:"attr,name"` or `jsonapi:"attr,email,omitempty"`
	TagValueAttribute = "attr"

	// TagValueRelationship indicates a field should be marshaled as a JSON:API relationship.
	// Format: `jsonapi:"relation,relationship-name[,omitempty]"`
	// Example:
	//
	// 	`jsonapi:"relation,posts"` or `jsonapi:"relation,profile,omitempty"`
	TagValueRelationship = "relation"

	// TagOptionOmitEmpty is a tag option that causes empty/zero values to be omitted
	// during marshaling. Can be used with both attributes and relationships.
	// Example:
	//
	// 	`jsonapi:"attr,email,omitempty"`
	TagOptionOmitEmpty = "omitempty"

	// TagOptionReadOnly is a tag option that prevents a field from being unmarshaled
	// unless the [PermitReadOnly] option is used. Read-only fields are still marshaled normally.
	// Can be used with both attributes and relationships.
	// Example:
	//
	// 	`jsonapi:"attr,created_at,readonly"` or `jsonapi:"relation,author,readonly"`
	TagOptionReadOnly = "readonly"

	// TagValueIgnore causes a field to be ignored during marshaling and unmarshaling.
	// Format: `jsonapi:"-"`
	TagValueIgnore = "-"
)

// Document represents the top-level JSON:API document structure.
type Document struct {
	Meta     map[string]interface{} `json:"meta,omitempty"`
	Data     PrimaryData            `json:"data,omitempty"`
	Errors   []Error                `json:"errors,omitempty"`
	Links    map[string]Link        `json:"links,omitempty"`
	Included []Resource             `json:"included,omitempty"`
}

// Resource represents a JSON:API resource object.
type Resource struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	Meta          map[string]interface{}  `json:"meta,omitempty"`
	Attributes    map[string]interface{}  `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Links         map[string]Link         `json:"links,omitempty"`
}

// Ref returns a resource reference (only ID and Type, no attributes/relationships).
func (r Resource) Ref() Resource {
	return Resource{
		ID:   r.ID,
		Type: r.Type,
	}
}

// ApplySparseFieldsets filters the resource's attributes to only include the specified fields.
// If fields is empty, all attributes are retained. Otherwise, only attributes whose names
// match one of the provided fields are kept, and all other attributes are removed.
func (r *Resource) ApplySparseFieldsets(fields []string) {
	// If no fields specified, keep all attributes
	if len(fields) == 0 {
		return
	}

	// Create a map for O(1) field lookups
	fieldMap := make(map[string]struct{})
	for _, field := range fields {
		fieldMap[field] = struct{}{}
	}

	// Filter attributes to only keep specified fields
	filteredAttrs := make(map[string]interface{})
	for key, value := range r.Attributes {
		if _, exists := fieldMap[key]; exists {
			filteredAttrs[key] = value
		}
	}
	r.Attributes = filteredAttrs
}

// PrimaryData represents the primary data in a JSON:API document.
// It can be a single resource, multiple resources, or null.
type PrimaryData struct {
	data interface{} // nil, Resource, or []Resource
}

// SingleResource creates primary data with a single resource.
func SingleResource(resource Resource) PrimaryData {
	return PrimaryData{data: resource}
}

// MultiResource creates primary data with multiple resources.
func MultiResource(resources ...Resource) PrimaryData {
	// Ensure we always have a non-nil slice, even if empty
	if resources == nil {
		resources = []Resource{}
	}
	return PrimaryData{data: resources}
}

// NullResource creates null primary data.
func NullResource() PrimaryData {
	return PrimaryData{data: nil}
}

// Null returns true if the data is null.
func (pd PrimaryData) Null() bool {
	return pd.data == nil
}

// One returns the single resource and true if data contains one resource.
func (pd PrimaryData) One() (Resource, bool) {
	if resource, ok := pd.data.(Resource); ok {
		return resource, true
	}
	return Resource{}, false
}

// Many returns the resource slice and true if data contains multiple resources.
func (pd PrimaryData) Many() ([]Resource, bool) {
	if resources, ok := pd.data.([]Resource); ok {
		return resources, true
	}
	return nil, false
}

// Iter returns an iterator over the resources.
func (pd *PrimaryData) Iter() iter.Seq[*Resource] {
	return func(yield func(*Resource) bool) {
		if resource, ok := pd.data.(Resource); ok {
			yield(&resource)
			pd.data = resource
		} else if resources, ok := pd.data.([]Resource); ok {
			for i, resource := range resources {
				next := yield(&resource)
				resources[i] = resource
				if !next {
					return
				}
			}
		}
	}
}

// MarshalJSON implements json.Marshaler for PrimaryData.
func (pd PrimaryData) MarshalJSON() ([]byte, error) {
	return json.Marshal(pd.data)
}

// UnmarshalJSON implements json.Unmarshaler for PrimaryData.
func (pd *PrimaryData) UnmarshalJSON(data []byte) error {
	// Check if it's null first
	if string(data) == "null" {
		pd.data = nil
		return nil
	}

	// Try to unmarshal as a single resource first
	var resource Resource
	if err := json.Unmarshal(data, &resource); err == nil {
		pd.data = resource
		return nil
	}

	// Try to unmarshal as an array of resources
	var resources []Resource
	if err := json.Unmarshal(data, &resources); err == nil {
		pd.data = resources
		return nil
	}

	return json.Unmarshal(data, &pd.data)
}

// Relationship represents a JSON:API relationship object.
type Relationship struct {
	Meta  map[string]interface{} `json:"meta,omitempty"`
	Links map[string]Link        `json:"links,omitempty"`
	Data  PrimaryData            `json:"data,omitempty"`
}

// Link represents a JSON:API link object.
type Link struct {
	Href string                 `json:"href,omitempty"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

func (l Link) MarshalJSON() ([]byte, error) {
	// If the link has no href, it's a null link
	if l.Href == "" {
		return []byte("null"), nil
	}

	// If there is no meta, return only the link.
	if len(l.Meta) == 0 {
		return json.Marshal(l.Href)
	}

	// Otherwise, marshal the link as a map
	return json.Marshal(map[string]interface{}{
		"href": l.Href,
		"meta": l.Meta,
	})
}

func (l *Link) UnmarshalJSON(data []byte) error {
	// Check if it's null first
	if string(data) == "null" {
		l.Href = ""
		l.Meta = nil
		return nil
	}

	// Try to unmarshal as a map
	if bytes.HasPrefix(data, []byte("{")) {
		var link struct {
			Href string                 `json:"href"`
			Meta map[string]interface{} `json:"meta"`
		}

		err := json.Unmarshal(data, &link)

		l.Href = link.Href
		l.Meta = link.Meta

		return err
	}

	// Try to unmarshal as string
	l.Meta = nil
	return json.Unmarshal(data, &l.Href)
}

// Error represents a JSON:API error object.
type Error struct {
	ID     string                 `json:"id,omitempty"`
	Status string                 `json:"status"`
	Code   string                 `json:"code"`
	Title  string                 `json:"title"`
	Detail string                 `json:"detail"`
	Source map[string]interface{} `json:"source,omitempty"`
	Links  map[string]interface{} `json:"links,omitempty"`
}

// NewErrorDocument creates a new [Document] to represent errors in a JSON:API compliant format.
// This function handles different types of errors and converts them into the appropriate Document structure.
//
//   - When the provided error wraps or is of type [Error], the function will use that error directly in the document
//   - If the error wraps or is of type [MultiError], all of the contained errors will be included in the document.
//
// For any other error type, the function creates a generic error entry using the error's message as the detail field.
// This ensures that even standard Go errors can be represented in the JSON:API format.
//
// If a nil error is provided, the function creates a document with a generic "Unknown error" message.
// This prevents returning empty error documents.
func NewErrorDocument(err error) *Document {
	var (
		doc          Document
		jsonErr      Error
		jsonMultiErr MultiError
	)

	if errors.As(err, &jsonErr) {
		doc.Errors = []Error{jsonErr}
	} else if errors.As(err, &jsonMultiErr) {
		doc.Errors = jsonMultiErr
	} else if err != nil {
		doc.Errors = append(doc.Errors, Error{Detail: err.Error()})
	} else {
		doc.Errors = append(doc.Errors, Error{Detail: "Unknown error"})
	}

	return &doc
}

// Error returns a string representation of the error.
// The returned string will include the title, detail, and code if they are available.
// If only the title and detail are available, it returns them formatted as "title: detail".
// If only the detail is available, it returns just the detail string.
func (e Error) Error() string {
	if e.Code != "" && e.Title != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Title, e.Detail, e.Code)
	}

	if e.Title != "" {
		return fmt.Sprintf("%s: %s", e.Title, e.Detail)
	}

	return e.Detail
}

// MultiError represents a collection of JSON:API errors that can be combined into a single error.
// It implements the error interface and provides a way to handle multiple errors as one.
type MultiError []Error

// Error returns a string representation of multiple errors combined into one.
// If the MultiError is empty, it panics with "multi error is empty".
// If the MultiError contains only one error, it returns that error's string representation.
// For multiple errors, it joins them together using errors.Join and returns the combined string.
func (me MultiError) Error() string {
	if len(me) == 0 {
		panic("multi error is empty")
	}

	if len(me) == 1 {
		return me[0].Error()
	}

	errs := make([]error, len(me))
	for i := range me {
		errs[i] = me[i]
	}

	return errors.Join(errs...).Error()
}

// Marshaling interfaces

// ResourceMarshaler allows types to provide custom JSON:API resource marshaling.
type ResourceMarshaler interface {
	MarshalJSONAPIResource(ctx context.Context) (Resource, error)
}

// LinksMarshaler allows types to provide custom links marshaling.
type LinksMarshaler interface {
	MarshalJSONAPILinks(ctx context.Context) (map[string]Link, error)
}

// MetaMarshaler allows types to provide custom meta marshaling.
type MetaMarshaler interface {
	MarshalJSONAPIMeta(ctx context.Context) (map[string]interface{}, error)
}

// RelationshipLinksMarshaler allows types to provide custom relationship links marshaling.
type RelationshipLinksMarshaler interface {
	MarshalJSONAPIRelationshipLinks(ctx context.Context, name string) (map[string]Link, error)
}

// RelationshipMetaMarshaler allows types to provide custom relationship meta marshaling.
type RelationshipMetaMarshaler interface {
	MarshalJSONAPIRelationshipMeta(ctx context.Context, name string) (map[string]interface{}, error)
}

// Unmarshaling interfaces

// ResourceUnmarshaler allows types to provide custom JSON:API resource unmarshaling.
type ResourceUnmarshaler interface {
	UnmarshalJSONAPIResource(ctx context.Context, resource Resource) error
}

// LinksUnmarshaler allows types to provide custom links unmarshaling.
type LinksUnmarshaler interface {
	UnmarshalJSONAPILinks(ctx context.Context, links map[string]Link) error
}

// MetaUnmarshaler allows types to provide custom meta unmarshaling.
type MetaUnmarshaler interface {
	UnmarshalJSONAPIMeta(ctx context.Context, meta map[string]interface{}) error
}

// RelationshipLinksUnmarshaler allows types to provide custom relationship links unmarshaling.
type RelationshipLinksUnmarshaler interface {
	UnmarshalJSONAPIRelationshipLinks(ctx context.Context, name string, links map[string]Link) error
}

// RelationshipMetaUnmarshaler allows types to provide custom relationship meta unmarshaling.
type RelationshipMetaUnmarshaler interface {
	UnmarshalJSONAPIRelationshipMeta(ctx context.Context, name string, meta map[string]interface{}) error
}
