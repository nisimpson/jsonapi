// Package jsonapi provides JSON:API specification compliant marshaling and unmarshaling functionality.
// It implements the JSON:API v1.0 specification for building APIs that follow REST conventions
// with standardized document structure, resource identification, and relationship handling.
package jsonapi

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalFunc defines the signature for JSON marshaling functions.
type MarshalFunc = func(v interface{}) ([]byte, error)

// UnmarshalFunc defines the signature for JSON unmarshaling functions.
type UnmarshalFunc = func(data []byte, v interface{}) error

var (
	jsonMarshal   MarshalFunc   = json.Marshal   // Default JSON marshaler
	jsonUnmarshal UnmarshalFunc = json.Unmarshal // Default JSON unmarshaler
)

// SetJSONMarshaler replaces the default JSON marshaling function with a custom implementation.
// This allows users to integrate custom JSON libraries or add preprocessing logic.
func SetJSONMarshaler(m MarshalFunc) {
	jsonMarshal = m
}

// SetJSONUnmarshaler replaces the default JSON unmarshaling function with a custom implementation.
// This allows users to integrate custom JSON libraries or add preprocessing logic.
func SetJSONUnmarshaler(u UnmarshalFunc) {
	jsonUnmarshal = u
}

// Document represents the top-level JSON:API document structure as defined in the specification.
// It contains the primary data, metadata, links, errors, and included resources.
type Document struct {
	Links    map[string]Link        `json:"links,omitempty"`    // Top-level links object
	Meta     map[string]interface{} `json:"meta,omitempty"`     // Top-level meta information
	Errors   []*Error               `json:"errors,omitempty"`   // Array of error objects
	Data     *DocumentData          `json:"data,omitempty"`     // Primary data for the document
	Included []*Resource            `json:"included,omitempty"` // Array of included resource objects
}

// DocumentData represents the primary data of a JSON:API [Document].
// It can contain either a single [Resource] or an array of resources.
type DocumentData struct {
	one    Resource   // Single resource data
	many   []Resource // Multiple resource data
	isMany bool       // Flag indicating if data represents multiple resources
}

// MarshalJSON implements the [json.Marshaler] interface for [DocumentData].
// It serializes the data as either a single [Resource] object, an array of resources,
// or null depending on the content and type.
func (d DocumentData) MarshalJSON() ([]byte, error) {
	if d.isMany {
		if len(d.many) == 0 {
			return jsonMarshal([]Ref{})
		}
		return jsonMarshal(d.many)
	}

	if d.one.ID == "" {
		return []byte("null"), nil
	}

	return jsonMarshal(d.one)
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for [DocumentData].
// It determines whether the incoming data represents a single [Resource] or multiple resources
// and unmarshals accordingly.
func (d *DocumentData) UnmarshalJSON(data []byte) error {
	if data[0] == '[' {
		d.isMany = true
		return jsonUnmarshal(data, &d.many)
	}

	if bytes.HasPrefix(data, []byte("null")) {
		d.isMany = false
		return nil
	}

	return jsonUnmarshal(data, &d.one)
}

// Resource represents a JSON:API resource object containing identification,
// attributes, relationships, links, and metadata.
type Resource struct {
	ID            string                   `json:"id,omitempty"`            // Unique identifier for the resource
	Type          string                   `json:"type,omitempty"`          // Resource type identifier
	Attributes    json.RawMessage          `json:"attributes,omitempty"`    // Resource attributes as raw JSON
	Relationships map[string]*Relationship `json:"relationships,omitempty"` // Related resources
	Links         map[string]Link          `json:"links,omitempty"`         // Resource-specific links
	Meta          map[string]interface{}   `json:"meta,omitempty"`          // Resource-specific metadata
}

// Relationship represents a JSON:API relationship object that describes
// the links between resources and optionally includes related resource data.
type Relationship struct {
	Data  *RelationshipData      `json:"data,omitempty"`  // Related resource identifiers
	Links map[string]Link        `json:"links,omitempty"` // Relationship-specific links
	Meta  map[string]interface{} `json:"meta,omitempty"`  // Relationship-specific metadata
}

func (r Relationship) hoistToPrimary(doc *Document) {
	doc.Links = r.Links
	doc.Meta = r.Meta
	if r.Data == nil {
		return
	}
	r.Data.hoistToPrimary(doc)
}

// RelationshipData represents the data portion of a [Relationship].
// It can contain either a single [Ref] or multiple resource references.
type RelationshipData struct {
	one    Ref   // Single resource reference
	many   []Ref // Multiple resource references
	isMany bool  // Flag indicating if relationship contains multiple references
}

func (r RelationshipData) hoistToPrimary(doc *Document) {
	doc.Data = &DocumentData{}
	if r.isMany {
		doc.Data.isMany = true
		for _, ref := range r.many {
			doc.Data.many = append(doc.Data.many, Resource{ID: ref.ID, Type: ref.Type})
		}
	} else {
		doc.Data.one = Resource{ID: r.one.ID, Type: r.one.Type}
	}
}

// MarshalJSON implements the [json.Marshaler] interface for [RelationshipData].
// It serializes the relationship data as either a single resource reference,
// an array of references, or null depending on the content and type.
func (r RelationshipData) MarshalJSON() ([]byte, error) {
	if r.isMany {
		if len(r.many) == 0 {
			return jsonMarshal([]Ref{})
		}
		return jsonMarshal(r.many)
	}

	if r.one.ID == "" {
		return []byte("null"), nil
	}

	return jsonMarshal(r.one)
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for [RelationshipData].
// It determines whether the incoming data represents a single reference or multiple references
// and unmarshals accordingly.
func (r *RelationshipData) UnmarshalJSON(data []byte) error {
	if data[0] == '[' {
		r.isMany = true
		return jsonUnmarshal(data, &r.many)
	}

	if bytes.HasPrefix(data, []byte("null")) {
		r.isMany = false
		return nil
	}

	return jsonUnmarshal(data, &r.one)
}

// Ref represents a JSON:API [ResourceIdentifier] object that provides
// a minimal representation of a [Resource] with just its type and identifier.
type Ref struct {
	Type string                 `json:"type,omitempty"` // Resource type identifier
	ID   string                 `json:"id,omitempty"`   // Unique resource identifier
	Meta map[string]interface{} `json:"meta,omitempty"` // Reference-specific metadata
}

// Ref also implements the [ResourceIdentifier] interface for [Resource] identification.
var _ ResourceIdentifier = Ref{}

func (r Ref) ResourceID() string   { return r.ID }
func (r Ref) ResourceType() string { return r.Type }

// Error represents a JSON:API error object that provides detailed information
// about problems encountered during request processing.
type Error struct {
	ID     string                 `json:"id,omitempty"`     // Unique identifier for this error instance
	Status string                 `json:"status,omitempty"` // HTTP status code as string
	Code   string                 `json:"code,omitempty"`   // Application-specific error code
	Title  string                 `json:"title,omitempty"`  // Short, human-readable summary
	Detail string                 `json:"detail,omitempty"` // Human-readable explanation
	Source ErrorSource            `json:"source,omitempty"` // Object containing references to error source
	Meta   map[string]interface{} `json:"meta,omitempty"`   // Error-specific metadata
}

func (e *Error) Error() string {
	if e.Title != "" && e.Code != "" {
		return fmt.Sprintf("(%s) %s: %s", e.Code, e.Title, e.Detail)
	}
	if e.Title != "" {
		return fmt.Sprintf("%s: %s", e.Title, e.Detail)
	}
	return e.Detail
}

// ErrorSource represents the source of an [Error], indicating where in the request
// [Document] or parameter the error originated.
type ErrorSource struct {
	Pointer   string `json:"pointer,omitempty"`   // JSON Pointer to the associated entity in request document
	Parameter string `json:"parameter,omitempty"` // String indicating which URI query parameter caused error
}

// Link represents a JSON:API link object that can be either a simple URL string
// or an object containing href and metadata.
type Link struct {
	Href string                 `json:"href,omitempty"` // Target URI for the link
	Meta map[string]interface{} `json:"meta,omitempty"` // Link-specific metadata
}

// MarshalJSON implements the [json.Marshaler] interface for [Link].
// It serializes the link as a simple string if no metadata is present,
// or as an object containing href and meta if metadata exists.
func (l Link) MarshalJSON() ([]byte, error) {
	if len(l.Meta) == 0 {
		return jsonMarshal(l.Href)
	}

	type link Link
	return jsonMarshal(link(l))
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for [Link].
// It handles both string URLs and link objects with href and metadata.
func (l *Link) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		return jsonUnmarshal(data, (*string)(&l.Href))
	}

	type link Link
	return jsonUnmarshal(data, (*link)(l))
}
