package jsonapi

import (
	"fmt"
	"reflect"
)

// ResourceIdentifier defines the interface that all JSON:API resources must implement
// to provide their unique identifier and type information.
type ResourceIdentifier interface {
	// ResourceID returns the unique identifier for this resource.
	ResourceID() string
	// ResourceType returns the type name for this resource.
	ResourceType() string
}

// LinksMarshaler defines the interface for resources that can provide link information
// during marshaling operations.
type LinksMarshaler interface {
	// MarshalLinks returns links associated with this resource.
	MarshalLinks() map[string]Link
}

// MetaMarshaler defines the interface for resources that can provide metadata
// during marshaling operations.
type MetaMarshaler interface {
	// MarshalMeta returns metadata associated with this resource.
	MarshalMeta() map[string]interface{}
}

// RelationshipMarshaler defines the interface for resources that have relationships
// with other resources and can provide relationship information during marshaling.
type RelationshipMarshaler interface {
	ResourceIdentifier
	// Relationships returns map of relationship names to their types.
	Relationships() map[string]RelationType
	// MarshalRef returns related resources for the given relationship name.
	MarshalRef(name string) []ResourceIdentifier
}

// OneRef creates a slice containing a single [ResourceIdentifier]. This is a helper function
// for implementing one-to-one relationships. If the input reference is nil, it returns nil.
// An input reference with an empty ResourceID will also return nil.
// This is useful when implementing the [RelationshipMarshaler.MarshalRef] method.
func OneRef(ref ResourceIdentifier) []ResourceIdentifier {
	if ref == nil {
		return nil
	}
	if ref.ResourceID() == "" {
		return nil
	}
	return []ResourceIdentifier{ref}
}

// ManyRef creates a slice of [ResourceIdentifier] from a variadic number of references.
// This is a helper function for implementing one-to-many relationships. It accepts
// any type T that implements ResourceIdentifier. The function allocates a new slice
// with the exact capacity needed and copies all references into it.
// This is useful when implementing the [RelationshipMarshaler.MarshalRef] method.
func ManyRef[T ResourceIdentifier](refs ...T) []ResourceIdentifier {
	items := make([]ResourceIdentifier, 0, len(refs))
	for _, item := range refs {
		items = append(items, item)
	}
	return items
}

// RelationshipLinksMarshaler defines the interface for resources that can provide
// links specific to their relationships during marshaling.
type RelationshipLinksMarshaler interface {
	RelationshipMarshaler
	// MarshalRefLinks returns links for the specified relationship.
	MarshalRefLinks(name string) map[string]Link
}

// RelationshipMetaMarshaler defines the interface for resources that can provide
// metadata specific to their relationships during marshaling.
type RelationshipMetaMarshaler interface {
	RelationshipMarshaler
	// MarshalRefMeta returns metadata for the specified relationship.
	MarshalRefMeta(name string) map[string]interface{}
}

// RelationType represents the type of relationship between resources.
type RelationType int

const (
	RelationToOne     RelationType = iota // One-to-one relationship
	RelationToMany                        // One-to-many relationship
	RelationLinksOnly                     // Relationship with links only, no data
)

// Marshal converts Go values into JSON:API compliant JSON documents.
// It accepts single implementations of [ResourceIdentifier], slices,
// nil values, or [Document] instances and returns a properly formatted JSON:API document
// with optional configuration.
func Marshal(data interface{}, opts ...Options) ([]byte, error) {
	options := applyOptions(opts)
	doc, err := marshalValue(data, &options)
	if err != nil {
		return nil, err
	}
	return jsonMarshal(doc)
}

// MarshalRef marshals a specific relationship from a resource into a JSON:API document.
// This is useful for relationship endpoints that return relationship data without
// the parent resource.
//
// The function extracts the specified relationship from the resource and creates
// a [Document] with the relationship data as the primary data. This follows the
// JSON:API specification for relationship endpoints like GET /articles/1/relationships/author.
//
// Example usage:
//
//	// Marshal the "author" relationship from an article
//	data, err := jsonapi.MarshalRef(article, "author")
//
//	// With options
//	data, err := jsonapi.MarshalRef(article, "tags",
//		jsonapi.WithDefaultLinks("https://api.example.com"))
//
// The relationship must be defined in the resource's [RelationshipMarshaler.Relationships] method,
// otherwise an error is returned.
func MarshalRef(data RelationshipMarshaler, name string, opts ...Options) ([]byte, error) {
	var (
		options         = applyOptions(opts)
		relationship    = &Relationship{}
		defs            = data.Relationships()
		refType, exists = defs[name]
		doc             = &Document{}
	)

	if !exists {
		return nil, fmt.Errorf("relationship %s not found", name)
	}

	if err := marshalRelationship(data, name, refType, relationship, 0, &options); err != nil {
		return nil, fmt.Errorf("relationship %s: %w", name, err)
	}

	relationship.hoistToPrimary(doc)
	finalDoc, err := marshalDocument(doc, &options)
	if err != nil {
		return nil, err
	}
	return jsonMarshal(finalDoc)
}

// marshalValue determines the type of data being marshaled and delegates
// to the appropriate marshaling function.
func marshalValue(data interface{}, options *options) (*Document, error) {
	if doc, ok := data.(*Document); ok {
		return marshalDocument(doc, options)
	}
	if doc, ok := data.(Document); ok {
		return marshalDocument(&doc, options)
	}
	if data == nil {
		return marshalDocument(&Document{}, options)
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice:
		return marshalMany(data, options)
	case reflect.Struct, reflect.Ptr:
		return marshalOne(data, options)
	default:
		return nil, fmt.Errorf("Marshal only accepts slice, struct or ptr types")
	}
}

// marshalOne marshals a single resource into a JSON:API document.
func marshalOne(data any, options *options) (*Document, error) {
	id, ok := data.(ResourceIdentifier)
	if !ok {
		return nil, fmt.Errorf("data does not implement ResourceIdentifier")
	}

	var (
		doc = &Document{}
		res = Resource{}
	)

	if err := marshalResource(id, &res, 0, options); err != nil {
		return nil, err
	}
	doc.Data = &DocumentData{one: res}
	return marshalDocument(doc, options)
}

// marshalMany marshals a slice of resources into a JSON:API document.
func marshalMany(data any, options *options) (*Document, error) {
	var (
		val = reflect.ValueOf(data)
		res = make([]Resource, val.Len())
		doc = &Document{}
	)

	for idx := range res {
		k := val.Index(idx).Interface()
		if id, ok := k.(ResourceIdentifier); !ok {
			return nil, fmt.Errorf("all elements within the slice must implement ResourceIdentifier")
		} else if err := marshalResource(id, &res[idx], 0, options); err != nil {
			return nil, err
		}
	}

	doc.Data = &DocumentData{many: res, isMany: true}
	return marshalDocument(doc, options)
}

// marshalDocument applies top-level document options and finalizes the document structure.
func marshalDocument(doc *Document, options *options) (*Document, error) {
	if len(options.topLinks) > 0 {
		doc.Links = options.topLinks
	}
	if len(options.topMeta) > 0 {
		doc.Meta = options.topMeta
	}
	if len(options.errors) > 0 {
		doc.Errors = options.errors
	}

	if doc.Included != nil {
		for _, res := range doc.Included {
			doc.Included = append(doc.Included, res)
		}
	}

	return doc, nil
}

// marshalResource marshals a single resource identifier into a Resource object,
// including its attributes, links, metadata, and relationships.
func marshalResource(id ResourceIdentifier, res *Resource, depth int, options *options) error {
	res.ID = id.ResourceID()
	res.Type = id.ResourceType()

	attributes, err := jsonMarshal(id)
	if err != nil {
		return err
	}

	res.Attributes = attributes

	if marshaler, ok := id.(LinksMarshaler); ok {
		res.Links = marshaler.MarshalLinks()
	}

	if res.Links == nil && len(options.linkResolver) > 0 {
		res.Links = map[string]Link{}
	}

	for key, resolver := range options.linkResolver {
		if link, ok := resolver.ResolveResourceLink(key, id); ok {
			res.Links[key] = link
		}
	}

	if marshaler, ok := id.(MetaMarshaler); ok {
		res.Meta = marshaler.MarshalMeta()
	}

	if marshaler, ok := id.(RelationshipMarshaler); ok {
		res.Relationships = make(map[string]*Relationship)
		for name, reftype := range marshaler.Relationships() {
			rel := &Relationship{}
			res.Relationships[name] = rel
			if err := marshalRelationship(marshaler, name, reftype, rel, depth, options); err != nil {
				return err
			}
		}
	}

	return nil
}

// marshalRelationship marshals a single relationship, including its data, links, and metadata.
// It also handles included resources based on the current depth and options.
func marshalRelationship(id RelationshipMarshaler, name string, refType RelationType, res *Relationship, depth int, options *options) error {
	if marshaler, ok := id.(RelationshipLinksMarshaler); ok {
		res.Links = marshaler.MarshalRefLinks(name)
	}

	if res.Links == nil && len(options.linkResolver) > 0 {
		res.Links = map[string]Link{}
	}

	for key, resolver := range options.linkResolver {
		if link, ok := resolver.ResolveRelationshipLink(key, name, id); ok {
			res.Links[key] = link
		}
	}

	if marshaler, ok := id.(RelationshipMetaMarshaler); ok {
		res.Meta = marshaler.MarshalRefMeta(name)
	}
	if refType == RelationLinksOnly {
		return nil
	}

	refs := id.MarshalRef(name)
	if refType == RelationToOne && len(refs) > 0 {
		var (
			data = refs[0]
			ref  = Ref{ID: data.ResourceID(), Type: data.ResourceType()}
		)
		if marshaler, ok := data.(MetaMarshaler); ok {
			ref.Meta = marshaler.MarshalMeta()
		}
		res.Data = &RelationshipData{one: ref}
	}
	if refType == RelationToMany {
		res.Data = &RelationshipData{isMany: true}
		for _, data := range refs {
			ref := Ref{ID: data.ResourceID(), Type: data.ResourceType()}
			if marshaler, ok := data.(MetaMarshaler); ok {
				ref.Meta = marshaler.MarshalMeta()
			}
			res.Data.many = append(res.Data.many, ref)
		}
	}

	if depth >= options.maxIncludeDepth {
		return nil
	}

	for _, data := range refs {
		uid := resourceUID(data)
		if _, exists := options.includes[uid]; exists {
			continue
		}
		res := &Resource{}
		options.includes[uid] = res
		if err := marshalResource(data, res, depth+1, options); err != nil {
			return err
		}
	}

	return nil
}

// resourceUID generates a unique identifier for a resource based on its type and ID.
func resourceUID(id ResourceIdentifier) string {
	return id.ResourceType() + ":" + id.ResourceID()
}
