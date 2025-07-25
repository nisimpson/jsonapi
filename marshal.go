package jsonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MarshalOptions contains options for marshaling operations.
type MarshalOptions struct {
	marshaler      func(interface{}) ([]byte, error)
	modifyDocument []func(*Document)
	includeRelated bool
}

// WithMarshaler uses a custom JSON marshaler to serialize documents.
func WithMarshaler(fn func(interface{}) ([]byte, error)) func(*MarshalOptions) {
	return func(opts *MarshalOptions) {
		opts.marshaler = fn
	}
}

// SparseFieldsets returns a function that modifies [MarshalOptions] to apply sparse fieldsets to resources.
// The function takes a resourceType string to identify which resources to modify and a fields slice containing
// the field names to include in the output. When applied, this function will filter both the primary data
// and included resources to only include the specified fields for resources matching the given type.
func SparseFieldsets(resourceType string, fields []string) func(*MarshalOptions) {
	return func(mo *MarshalOptions) {
		mo.modifyDocument = append(mo.modifyDocument, func(d *Document) {
			if d.Data.Null() {
				return
			}
			for resource := range d.Data.Iter() {
				if resource.Type == resourceType {
					resource.ApplySparseFieldsets(fields)
				}
			}
			for i, resource := range d.Included {
				if resource.Type == resourceType {
					resource.ApplySparseFieldsets(fields)
					d.Included[i] = resource
				}
			}
		})
	}
}

// DocumentMeta returns a function that modifies MarshalOptions to add metadata to the JSON:API document.
// The provided meta map will be set as the top-level meta object in the resulting document.
// This is useful for adding custom metadata like pagination info or document-level statistics.
func DocumentMeta(meta map[string]interface{}) func(*MarshalOptions) {
	return func(mo *MarshalOptions) {
		mo.modifyDocument = append(mo.modifyDocument, func(d *Document) {
			d.Meta = meta
		})
	}
}

// DocumentLinks returns a function that modifies MarshalOptions to add links to the JSON:API document.
// The provided links map will be set as the top-level links object in the resulting document.
// This can be used to add pagination, self-referential, or related resource links to the document.
func DocumentLinks(links map[string]Link) func(*MarshalOptions) {
	return func(mo *MarshalOptions) {
		mo.modifyDocument = append(mo.modifyDocument, func(d *Document) {
			d.Links = links
		})
	}
}

// IncludeRelatedResources instructs the Marshaler to add any related resources
// found within the document's primary data to the included array.
func IncludeRelatedResources() func(*MarshalOptions) {
	return func(opts *MarshalOptions) {
		opts.includeRelated = true
	}
}

// Marshal marshals a Go struct into a JSON:API Resource using the default context.
func Marshal(out interface{}, opts ...func(*MarshalOptions)) ([]byte, error) {
	return MarshalWithContext(context.Background(), out, opts...)
}

// MarshalWithContext marshals a Go struct into a JSON:API Resource with a provided context.
func MarshalWithContext(ctx context.Context, out interface{}, opts ...func(*MarshalOptions)) ([]byte, error) {
	options := &MarshalOptions{
		marshaler: json.Marshal,
	}

	for _, opt := range opts {
		opt(options)
	}

	doc, err := MarshalDocument(ctx, out, opts...)
	if err == nil {
		return options.marshaler(doc)
	}

	return nil, err
}

// MarshalDocument converts a Go value into a JSON:API Document structure.
// It accepts a context for injection of request-scoped values in marshaling operations.
// Optional marshaling options can be provided to customize the marshaling behavior.
// It returns a Document pointer and any error encountered during marshaling.
// If the input is nil or a nil pointer, it returns an error.
//
// This function is the core marshaling function used by Marshal and MarshalWithContext.
// It can be used directly when you need access to the Document structure before serialization.
func MarshalDocument(ctx context.Context, out interface{}, opts ...func(*MarshalOptions)) (*Document, error) {
	options := &MarshalOptions{
		marshaler: json.Marshal,
	}

	for _, opt := range opts {
		opt(options)
	}

	if out == nil {
		return nil, fmt.Errorf("cannot marshal nil value")
	}

	// Check for nil pointer
	v := reflect.ValueOf(out)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil, fmt.Errorf("cannot marshal nil value")
	}

	doc, err := marshalToDocument(ctx, out, options)
	if err != nil {
		return nil, err
	}

	for _, mod := range options.modifyDocument {
		mod(doc)
	}

	return doc, nil
}

// marshalToDocument converts the input to a JSON:API document.
func marshalToDocument(ctx context.Context, out interface{}, opts *MarshalOptions) (*Document, error) {
	doc := &Document{}

	// Handle slice inputs
	if isSlice(out) {
		resources, included, err := marshalSlice(ctx, out)
		if err != nil {
			return nil, err
		}
		doc.Data = MultiResource(resources...)
		if opts.includeRelated {
			doc.Included = included
		}
		return doc, nil
	}

	// Handle single resource
	resource, included, err := marshalSingle(ctx, out)
	if err != nil {
		return nil, err
	}

	doc.Data = SingleResource(resource)
	if opts.includeRelated {
		doc.Included = included
	}

	return doc, nil
}

// marshalSlice marshals a slice of structs to resources.
func marshalSlice(ctx context.Context, out interface{}) ([]Resource, []Resource, error) {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Slice {
		return nil, nil, fmt.Errorf("expected slice, got %T", out)
	}

	var resources []Resource
	var included []Resource

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i).Interface()
		resource, relatedResources, err := marshalSingle(ctx, elem)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, resource)
		included = append(included, relatedResources...)
	}

	return resources, included, nil
}

// marshalSingle marshals a single struct to a resource.
func marshalSingle(ctx context.Context, out interface{}) (Resource, []Resource, error) {
	var resource Resource
	var included []Resource
	var err error

	// Check if the type implements ResourceMarshaler
	if marshaler, ok := out.(ResourceMarshaler); ok {
		resource, err = marshaler.MarshalJSONAPIResource(ctx)
		if err != nil {
			return Resource{}, nil, err
		}
	} else {
		// Use reflection-based marshaling
		resource, included, err = marshalWithReflection(ctx, out)
		if err != nil {
			return Resource{}, nil, err
		}
	}

	// Add custom links if the type implements LinksMarshaler
	if linksMarshaler, ok := out.(LinksMarshaler); ok {
		links, err := linksMarshaler.MarshalJSONAPILinks(ctx)
		if err != nil {
			return Resource{}, nil, err
		}
		resource.Links = links
	}

	// Add custom meta if the type implements MetaMarshaler
	if metaMarshaler, ok := out.(MetaMarshaler); ok {
		meta, err := metaMarshaler.MarshalJSONAPIMeta(ctx)
		if err != nil {
			return Resource{}, nil, err
		}
		resource.Meta = meta
	}

	return resource, included, nil
}

// marshalWithReflection uses reflection to marshal a struct based on tags.
func marshalWithReflection(ctx context.Context, out interface{}) (Resource, []Resource, error) {
	v := reflect.ValueOf(out)

	// Dereference pointer if necessary
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return Resource{}, nil, fmt.Errorf("cannot marshal nil pointer")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return Resource{}, nil, fmt.Errorf("expected struct, got %T", out)
	}

	resource := Resource{
		Attributes:    make(map[string]interface{}),
		Relationships: make(map[string]Relationship),
	}

	var included []Resource
	t := v.Type()

	// Process all fields including embedded ones
	fields := getAllFields(t)

	for _, fieldInfo := range fields {
		field := fieldInfo.Field
		fieldValue := getFieldValue(v, fieldInfo)

		tag := field.Tag.Get(StructTagName)
		if tag == "" {
			continue
		}

		parts := strings.Split(tag, ",")
		if len(parts) < 2 {
			continue
		}

		tagType := parts[0]
		name := parts[1]
		options := parts[2:]

		// Check omitempty option
		if contains(options, TagOptionOmitEmpty) && isZeroValue(fieldValue) {
			continue
		}

		switch tagType {
		case TagValuePrimary:
			resource.Type = name
			resource.ID = fmt.Sprintf("%v", fieldValue.Interface())
		case TagValueAttribute:
			resource.Attributes[name] = fieldValue.Interface()
		case TagValueRelationship:
			rel, relatedResources, err := marshalRelationship(ctx, fieldValue, options)
			if err != nil {
				return Resource{}, nil, err
			}
			resource.Relationships[name] = rel
			included = append(included, relatedResources...)
		}
	}

	return resource, included, nil
}

// marshalRelationship marshals a relationship field.
func marshalRelationship(ctx context.Context, fieldValue reflect.Value, options []string) (Relationship, []Resource, error) {
	rel := Relationship{}
	var included []Resource

	// Zero values that are not slices...
	if isZeroValue(fieldValue) {
		if contains(options, TagOptionOmitEmpty) {
			return rel, nil, nil
		}
		if fieldValue.Kind() != reflect.Slice {
			rel.Data = NullResource()
			return rel, nil, nil
		}
	}

	// Handle slice relationships
	if fieldValue.Kind() == reflect.Slice {
		var resources []Resource
		for i := 0; i < fieldValue.Len(); i++ {
			elem := fieldValue.Index(i).Interface()
			resource, relatedResources, err := marshalSingle(ctx, elem)
			if err != nil {
				return Relationship{}, nil, err
			}
			resources = append(resources, resource.Ref())
			included = append(included, resource)
			included = append(included, relatedResources...)
		}
		rel.Data = MultiResource(resources...)
	} else {
		// Handle single relationships
		elem := fieldValue.Interface()
		resource, relatedResources, err := marshalSingle(ctx, elem)
		if err != nil {
			return Relationship{}, nil, err
		}
		rel.Data = SingleResource(resource.Ref())
		included = append(included, resource)
		included = append(included, relatedResources...)
	}

	return rel, included, nil
}

// Helper functions

// fieldInfo holds information about a field and how to access it.
type fieldInfo struct {
	Field reflect.StructField
	Path  []int // Path to the field through embedded structs
}

// getAllFields returns all fields including embedded struct fields.
func getAllFields(t reflect.Type) []fieldInfo {
	var fields []fieldInfo
	collectFields(t, nil, &fields)
	return fields
}

// collectFields recursively collects fields from a struct type.
func collectFields(t reflect.Type, path []int, fields *[]fieldInfo) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		currentPath := append(path, i)

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			// Embedded struct - recursively collect its fields
			collectFields(field.Type, currentPath, fields)
		} else {
			*fields = append(*fields, fieldInfo{
				Field: field,
				Path:  currentPath,
			})
		}
	}
}

// getFieldValue gets the value of a field using the path information.
func getFieldValue(v reflect.Value, info fieldInfo) reflect.Value {
	fieldValue := v
	for _, index := range info.Path {
		fieldValue = fieldValue.Field(index)
	}
	return fieldValue
}

// isSlice checks if the value is a slice.
func isSlice(v interface{}) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	return rv.Kind() == reflect.Slice
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
