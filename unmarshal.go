package jsonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// UnmarshalOptions contains options for unmarshaling operations.
type UnmarshalOptions struct {
	unmarshaler          func([]byte, interface{}) error
	strictMode           bool
	populateFromIncluded bool
}

// WithUnmarshaler uses a custom JSON unmarshaler to deserialize documents.
func WithUnmarshaler(fn func([]byte, interface{}) error) func(*UnmarshalOptions) {
	return func(opts *UnmarshalOptions) {
		opts.unmarshaler = fn
	}
}

// StrictMode enables strict mode unmarshaling which returns errors for invalid JSON:API structure.
func StrictMode() func(*UnmarshalOptions) {
	return func(opts *UnmarshalOptions) {
		opts.strictMode = true
	}
}

// PopulateFromIncluded instructs the Unmarshaler to populate relationship fields from the included array when available.
func PopulateFromIncluded() func(*UnmarshalOptions) {
	return func(opts *UnmarshalOptions) {
		opts.populateFromIncluded = true
	}
}

// Unmarshal unmarshals JSON:API data into a Go struct using the default context.
func Unmarshal(data []byte, out interface{}, opts ...func(*UnmarshalOptions)) error {
	return UnmarshalWithContext(context.Background(), data, out, opts...)
}

// UnmarshalWithContext unmarshals JSON:API data into a Go struct with a provided context.
func UnmarshalWithContext(ctx context.Context, data []byte, out interface{}, opts ...func(*UnmarshalOptions)) error {
	if _, ok := out.(*Document); ok {
		return fmt.Errorf("call UnmarshalDocument() to unmarshal into a document")
	}

	options := &UnmarshalOptions{
		unmarshaler: json.Unmarshal,
	}

	for _, opt := range opts {
		opt(options)
	}

	if out == nil {
		return fmt.Errorf("cannot unmarshal into nil value")
	}

	// Check that out is a pointer
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("out parameter must be a pointer, got %T", out)
	}

	if v.IsNil() {
		return fmt.Errorf("cannot unmarshal into nil pointer")
	}

	// Parse the JSON:API document
	var doc Document
	if err := options.unmarshaler(data, &doc); err != nil {
		return fmt.Errorf("failed to unmarshal JSON:API document: %w", err)
	}

	return unmarshalFromDocument(ctx, &doc, out, options)
}

// UnmarshalDocument unmarshals JSON:API data into a Document structure without converting to Go structs.
func UnmarshalDocument(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON:API document: %w", err)
	}
	return &doc, nil
}

// unmarshalFromDocument unmarshals a Document into the target struct.
func unmarshalFromDocument(ctx context.Context, doc *Document, out interface{}, opts *UnmarshalOptions) error {
	v := reflect.ValueOf(out).Elem()
	t := v.Type()

	// Handle slice targets
	if t.Kind() == reflect.Slice {
		return unmarshalSliceFromDocument(ctx, doc, v, opts)
	}

	// Handle single struct targets
	if doc.Data.Null() {
		// Set to zero value for null data
		v.Set(reflect.Zero(t))
		return nil
	}

	resource, ok := doc.Data.One()
	if !ok {
		// Try to get the first resource from many
		resources, ok := doc.Data.Many()
		if !ok || len(resources) == 0 {
			return fmt.Errorf("expected single resource but got none")
		}
		resource = resources[0]
	}

	return unmarshalResource(ctx, resource, v, doc.Included, opts)
}

// unmarshalSliceFromDocument unmarshals a Document into a slice target.
func unmarshalSliceFromDocument(ctx context.Context, doc *Document, v reflect.Value, opts *UnmarshalOptions) error {
	if doc.Data.Null() {
		// Set to nil slice for null data
		v.Set(reflect.Zero(v.Type()))
		return nil
	}

	resources, ok := doc.Data.Many()
	if !ok {
		// Try to get single resource and make it a slice
		resource, ok := doc.Data.One()
		if !ok {
			return fmt.Errorf("expected resource array but got none")
		}
		resources = []Resource{resource}
	}

	// Create slice of appropriate size
	elemType := v.Type().Elem()
	slice := reflect.MakeSlice(v.Type(), len(resources), len(resources))

	for i, resource := range resources {
		elem := slice.Index(i)
		if elemType.Kind() == reflect.Ptr {
			// Create new pointer element
			newElem := reflect.New(elemType.Elem())
			if err := unmarshalResource(ctx, resource, newElem.Elem(), doc.Included, opts); err != nil {
				return err
			}
			elem.Set(newElem)
		} else {
			// Direct element
			if err := unmarshalResource(ctx, resource, elem, doc.Included, opts); err != nil {
				return err
			}
		}
	}

	v.Set(slice)
	return nil
}

// unmarshalResource unmarshals a single resource into a struct value.
func unmarshalResource(ctx context.Context, resource Resource, v reflect.Value, included []Resource, opts *UnmarshalOptions) error {
	// Check if the type implements ResourceUnmarshaler
	if v.CanAddr() {
		if unmarshaler, ok := v.Addr().Interface().(ResourceUnmarshaler); ok {
			return unmarshaler.UnmarshalJSONAPIResource(ctx, resource)
		}
	}

	// Use reflection-based unmarshaling
	return unmarshalWithReflection(ctx, resource, v, included, opts)
}

// unmarshalWithReflection uses reflection to unmarshal a resource based on struct tags.
func unmarshalWithReflection(ctx context.Context, resource Resource, v reflect.Value, included []Resource, opts *UnmarshalOptions) error {
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", t.Kind())
	}

	// Get all fields including embedded ones
	fields := getAllFields(t)

	for _, fieldInfo := range fields {
		field := fieldInfo.Field
		fieldValue := getFieldValue(v, fieldInfo)

		if !fieldValue.CanSet() {
			continue
		}

		tag := field.Tag.Get("jsonapi")
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

		switch tagType {
		case "primary":
			// Validate resource type
			if resource.Type != name {
				if opts.strictMode {
					return fmt.Errorf("resource type mismatch: expected %s, got %s", name, resource.Type)
				}
			}
			// Set the ID
			if err := setFieldValue(fieldValue, resource.ID); err != nil {
				return fmt.Errorf("failed to set primary field %s: %w", field.Name, err)
			}

		case "attr":
			// Set attribute value
			if attrValue, exists := resource.Attributes[name]; exists {
				if err := setFieldValue(fieldValue, attrValue); err != nil {
					return fmt.Errorf("failed to set attribute field %s: %w", field.Name, err)
				}
			}

		case "relation":
			// Handle relationship
			if rel, exists := resource.Relationships[name]; exists {
				if err := unmarshalRelationship(ctx, rel, fieldValue, included, opts, options); err != nil {
					return fmt.Errorf("failed to set relationship field %s: %w", field.Name, err)
				}
			}
		}
	}

	// Handle custom unmarshalers
	if v.CanAddr() {
		addr := v.Addr().Interface()

		// Links unmarshaler
		if linksUnmarshaler, ok := addr.(LinksUnmarshaler); ok && resource.Links != nil {
			if err := linksUnmarshaler.UnmarshalJSONAPILinks(ctx, resource.Links); err != nil {
				return err
			}
		}

		// Meta unmarshaler
		if metaUnmarshaler, ok := addr.(MetaUnmarshaler); ok && resource.Meta != nil {
			if err := metaUnmarshaler.UnmarshalJSONAPIMeta(ctx, resource.Meta); err != nil {
				return err
			}
		}
	}

	return nil
}

// unmarshalRelationship unmarshals a relationship field.
func unmarshalRelationship(ctx context.Context, rel Relationship, fieldValue reflect.Value, included []Resource, opts *UnmarshalOptions, fieldOptions []string) error {
	if rel.Data.Null() {
		// Set to zero value for null relationships
		fieldValue.Set(reflect.Zero(fieldValue.Type()))
		return nil
	}

	if fieldValue.Type().Kind() == reflect.Slice {
		// Handle slice relationships
		resources, ok := rel.Data.Many()
		if !ok {
			// Single resource in a slice relationship
			resource, ok := rel.Data.One()
			if !ok {
				return nil // No data
			}
			resources = []Resource{resource}
		}

		return unmarshalSliceRelationship(ctx, resources, fieldValue, included, opts)
	} else {
		// Handle single relationships
		resource, ok := rel.Data.One()
		if !ok {
			// Try to get first from many
			resources, ok := rel.Data.Many()
			if !ok || len(resources) == 0 {
				// Set to zero value
				fieldValue.Set(reflect.Zero(fieldValue.Type()))
				return nil
			}
			resource = resources[0]
		}

		return unmarshalSingleRelationship(ctx, resource, fieldValue, included, opts)
	}
}

// unmarshalSliceRelationship unmarshals a slice relationship.
func unmarshalSliceRelationship(ctx context.Context, resources []Resource, fieldValue reflect.Value, included []Resource, opts *UnmarshalOptions) error {
	elemType := fieldValue.Type().Elem()
	slice := reflect.MakeSlice(fieldValue.Type(), 0, len(resources))

	for _, resource := range resources {
		var elem reflect.Value

		if opts.populateFromIncluded {
			// Find full resource in included
			if fullResource := findIncludedResource(resource.ID, resource.Type, included); fullResource != nil {
				resource = *fullResource
			}
		}

		if elemType.Kind() == reflect.Ptr {
			elem = reflect.New(elemType.Elem())
			if err := unmarshalResource(ctx, resource, elem.Elem(), included, opts); err != nil {
				return err
			}
		} else {
			elem = reflect.New(elemType).Elem()
			if err := unmarshalResource(ctx, resource, elem, included, opts); err != nil {
				return err
			}
		}

		slice = reflect.Append(slice, elem)
	}

	fieldValue.Set(slice)
	return nil
}

// unmarshalSingleRelationship unmarshals a single relationship.
func unmarshalSingleRelationship(ctx context.Context, resource Resource, fieldValue reflect.Value, included []Resource, opts *UnmarshalOptions) error {
	if opts.populateFromIncluded {
		// Find full resource in included
		if fullResource := findIncludedResource(resource.ID, resource.Type, included); fullResource != nil {
			resource = *fullResource
		}
	}

	if fieldValue.Type().Kind() == reflect.Ptr {
		elem := reflect.New(fieldValue.Type().Elem())
		if err := unmarshalResource(ctx, resource, elem.Elem(), included, opts); err != nil {
			return err
		}
		fieldValue.Set(elem)
	} else {
		if err := unmarshalResource(ctx, resource, fieldValue, included, opts); err != nil {
			return err
		}
	}

	return nil
}

// findIncludedResource finds a resource in the included array by ID and type.
func findIncludedResource(id, resourceType string, included []Resource) *Resource {
	for _, resource := range included {
		if resource.ID == id && resource.Type == resourceType {
			return &resource
		}
	}
	return nil
}

// setFieldValue sets a field value with type conversion.
func setFieldValue(fieldValue reflect.Value, value interface{}) error {
	if value == nil {
		fieldValue.Set(reflect.Zero(fieldValue.Type()))
		return nil
	}

	valueReflect := reflect.ValueOf(value)
	fieldType := fieldValue.Type()

	// Direct assignment if types match
	if valueReflect.Type().AssignableTo(fieldType) {
		fieldValue.Set(valueReflect)
		return nil
	}

	// Type conversion
	if valueReflect.Type().ConvertibleTo(fieldType) {
		fieldValue.Set(valueReflect.Convert(fieldType))
		return nil
	}

	// Special cases for common conversions
	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(fmt.Sprintf("%v", value))
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intVal, err := convertToInt(value); err == nil {
			fieldValue.SetInt(intVal)
			return nil
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintVal, err := convertToUint(value); err == nil {
			fieldValue.SetUint(uintVal)
			return nil
		}

	case reflect.Float32, reflect.Float64:
		if floatVal, err := convertToFloat(value); err == nil {
			fieldValue.SetFloat(floatVal)
			return nil
		}

	case reflect.Bool:
		if boolVal, err := convertToBool(value); err == nil {
			fieldValue.SetBool(boolVal)
			return nil
		}

	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Pointer:
		// For complex types, try JSON marshaling/unmarshaling
		if jsonBytes, err := json.Marshal(value); err == nil {
			newValue := reflect.New(fieldType)
			if err := json.Unmarshal(jsonBytes, newValue.Interface()); err == nil {
				fieldValue.Set(newValue.Elem())
				return nil
			}
		}
	}

	return fmt.Errorf("cannot convert %T to %s", value, fieldType)
}

// Helper functions for type conversion
func convertToInt(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

func convertToUint(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case string:
		return strconv.ParseUint(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to uint", value)
	}
}

func convertToFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", value)
	}
}

func convertToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}
