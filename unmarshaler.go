package jsonapi

import (
	"fmt"
	"io"
	"reflect"
)

// ResourceUnmarshaler defines the interface that resources must implement
// to support unmarshaling from JSON:API documents.
type ResourceUnmarshaler interface {
	ResourceIdentifier
	// SetResourceID sets the resource ID during unmarshaling.
	SetResourceID(id string) error
}

// MetaUnmarshaler defines the interface for resources that can receive
// metadata during unmarshaling operations.
type MetaUnmarshaler interface {
	MetaMarshaler
	// UnmarshalMeta receives metadata during unmarshaling.
	UnmarshalMeta(meta map[string]interface{}) error
}

// LinksUnmarshaler defines the interface for resources that can receive
// link information during unmarshaling operations.
type LinksUnmarshaler interface {
	LinksMarshaler
	// UnmarshalLinks receives links during unmarshaling.
	UnmarshalLinks(links map[string]Link) error
}

// RelationshipUnmarshaler defines the interface for resources that can receive
// relationship data during unmarshaling operations.
type RelationshipUnmarshaler interface {
	RelationshipMarshaler
	// UnmarshalRef sets relationship data during unmarshaling.
	UnmarshalRef(name string, id string, meta map[string]interface{}) error
}

// RelationshipLinksUnmarshaler defines the interface for resources that can receive
// relationship-specific links during unmarshaling operations.
type RelationshipLinksUnmarshaler interface {
	RelationshipMarshaler
	// UnmarshalRefLinks sets relationship links during unmarshaling.
	UnmarshalRefLinks(name string, links map[string]Link) error
}

// RelationshipMetaUnmarshaler defines the interface for resources that can receive
// relationship-specific metadata during unmarshaling operations.
type RelationshipMetaUnmarshaler interface {
	RelationshipMarshaler
	// UnmarshalRefMeta sets relationship metadata during unmarshaling.
	UnmarshalRefMeta(name string, meta map[string]interface{}) error
}

// UnmarshalData unmarshals the data portion of a JSON:API [Document] into the provided target.
// The target must be a pointer to a struct or slice that implements the appropriate unmarshaler interfaces.
func (d Document) UnmarshalData(target interface{}, opts ...Options) error {
	options := applyOptions(opts)

	if d.Data == nil {
		return fmt.Errorf(`%w: no data to unmarshal`, io.EOF)
	}

	if d.Data.one.Type != "" {
		return unmarshalOne(d.Data.one, target, &options)
	}

	if len(d.Data.many) > 0 {
		return unmarshalMany(d.Data.many, target, &options)
	}

	return fmt.Errorf("%w: no data to unmarshal", io.EOF)
}

// Unmarshal parses JSON:API formatted data and stores the result in the target.
// The target must be a pointer to a struct or slice that implements [ResourceUnmarshaler].
func Unmarshal(data []byte, target interface{}, opts ...Options) error {
	if target == nil {
		return fmt.Errorf("target must not be nil")
	}

	if reflect.TypeOf(target).Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a ptr")
	}

	doc := &Document{}
	if in, ok := target.(*Document); ok {
		doc = in
	}

	err := jsonUnmarshal(data, doc)
	if err != nil {
		return err
	}

	return doc.UnmarshalData(target, opts...)
}

// unmarshalMany unmarshals an array of resources into a slice target.
func unmarshalMany(many []Resource, target interface{}, options *options) error {
	if target == nil {
		return fmt.Errorf("unmarshal target must not be nil")
	}

	if reflect.TypeOf(target).Kind() != reflect.Ptr {
		return fmt.Errorf("unmarshal target must be a ptr")
	}

	targetSlice := reflect.TypeOf(target).Elem()
	if targetSlice.Kind() != reflect.Slice {
		return fmt.Errorf("cannot unmarshal array to struct target %s", targetSlice)
	}

	targetType := targetSlice.Elem()
	targetPointer := reflect.ValueOf(target)
	targetValue := targetPointer.Elem()

	for idx, record := range many {
		targetRecord := reflect.New(targetType)
		err := unmarshalOne(record, targetRecord.Interface(), options)
		if err != nil {
			return fmt.Errorf("unmarshal resource %d: %w", idx, err)
		}
		targetValue = reflect.Append(targetValue, targetRecord.Elem())
	}

	targetPointer.Elem().Set(targetValue)
	return nil
}

// unmarshalOne unmarshals a single resource into the target struct.
func unmarshalOne(one Resource, target interface{}, options *options) error {
	id, ok := target.(ResourceUnmarshaler)
	if !ok {
		return fmt.Errorf("unmarshal target must implement ResourceUnmarshaler")
	}

	if options.validateType && id.ResourceType() != one.Type {
		return fmt.Errorf("resource type mismatch: %s != %s", id.ResourceType(), one.Type)
	}

	id.SetResourceID(one.ID)
	err := jsonUnmarshal(one.Attributes, target)
	if err != nil {
		return fmt.Errorf("unmarshal attributes: %w", err)
	}

	if unmarshaler, ok := id.(LinksUnmarshaler); ok {
		if err := unmarshaler.UnmarshalLinks(one.Links); err != nil {
			return fmt.Errorf("unmarshal links: %w", err)
		}
	}

	if unmarshaler, ok := id.(MetaUnmarshaler); ok {
		if err := unmarshaler.UnmarshalMeta(one.Meta); err != nil {
			return fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	if unmarshaler, ok := id.(RelationshipUnmarshaler); ok {
		for name, rel := range one.Relationships {
			if err := unmarshalRelationship(rel, name, unmarshaler, options); err != nil {
				return fmt.Errorf("unmarshal relationship %s: %w", name, err)
			}
		}
	}

	return nil
}

// UnmarshalRef extracts relationship data from a JSON:API [Document] and populates
// the target's specified [Relationship]. This is useful for relationship endpoint
// operations like PATCH /resources/1/relationships/tags.
func UnmarshalRef(data []byte, name string, target RelationshipUnmarshaler, opts ...Options) error {
	options := applyOptions(opts)

	doc := &Document{}
	err := jsonUnmarshal(data, doc)
	if err != nil {
		return err
	}

	var (
		relationships   = target.Relationships()
		relType, exists = relationships[name]
	)

	if !exists {
		return fmt.Errorf("relationship %q not found for resource %q", name, target.ResourceType())
	}

	if doc.Data == nil {
		// Check if this is a to-many relationship - null is not allowed for to-many
		if relType == RelationToMany {
			return fmt.Errorf("null data not allowed for to-many relationship %q: use empty array instead", name)
		}

		// Handle null data by clearing the to-one relationship
		return target.UnmarshalRef(name, "", nil)
	}

	// Create a temporary relationship object from the document data
	relData := &RelationshipData{isMany: doc.Data.isMany}

	if doc.Data.isMany {
		relData.many = make([]Ref, len(doc.Data.many))
		for i, res := range doc.Data.many {
			relData.many[i] = Ref{ID: res.ID, Type: res.Type}
		}
	} else {
		// For single relationships, just copy the ref data
		relData.one = Ref{ID: doc.Data.one.ID, Type: doc.Data.one.Type}
	}

	rel := &Relationship{Data: relData}

	return unmarshalRelationship(rel, name, target, &options)
}

// unmarshalRelationship unmarshals a single relationship into the target resource.
func unmarshalRelationship(rel *Relationship, name string, id RelationshipUnmarshaler, options *options) error {
	if unmarshaler, ok := id.(RelationshipLinksUnmarshaler); ok && len(rel.Links) > 0 {
		if err := unmarshaler.UnmarshalRefLinks(name, rel.Links); err != nil {
			return fmt.Errorf("unmarshal links: %w", err)
		}
	}

	if unmarshaler, ok := id.(RelationshipMetaUnmarshaler); ok && len(rel.Meta) > 0 {
		if err := unmarshaler.UnmarshalRefMeta(name, rel.Meta); err != nil {
			return fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	if rel.Data == nil {
		// nothing to unmarshal
		return nil
	}

	if rel.Data.isMany {
		for idx, one := range rel.Data.many {
			if err := id.UnmarshalRef(name, one.ID, one.Meta); err != nil {
				return fmt.Errorf("unmarshal ref %d: %w", idx, err)
			}
		}
		return nil
	}

	return id.UnmarshalRef(name, rel.Data.one.ID, rel.Data.one.Meta)
}
