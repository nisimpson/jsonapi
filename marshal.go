package jsonapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
)

// ResourceMarshaler is an interface that types can implement to provide custom
// JSON:API resource marshaling behavior. Types implementing this interface
// will have their MarshalJSONAPIResource method called instead of the default
// struct tag-based marshaling.
type ResourceMarshaler interface {
	// MarshalJSONAPIResource returns a JSON:API Resource representation of the type.
	// This method should populate the Resource with appropriate Type, ID, Attributes,
	// and Relationships based on the implementing type's data.
	MarshalJSONAPIResource(ctx context.Context) (Resource, error)
}

type LinksMarshaler interface {
	MarshalJSONAPILinks(ctx context.Context) (map[string]Link, error)
}

type MetaMarshaler interface {
	MarshalJSONAPIMeta(ctx context.Context) (map[string]interface{}, error)
}

type RelationshipLinksMarshaler interface {
	MarshalJSONAPIRelationshipLinks(ctx context.Context, name string) (map[string]Link, error)
}

type RelationshipMetaMarshaler interface {
	MarshalJSONAPIRelationshipMeta(ctx context.Context, name string) (map[string]interface{}, error)
}

type MarshalOptions struct {
	isRef bool
}

// MarshalResourceWithContext converts a Go struct into a JSON:API Resource representation.
// It supports two marshaling approaches:
//
//  1. Custom marshaling: If the input implements ResourceMarshaler interface,
//     it calls the MarshalJSONAPIResource method.
//
//  2. Struct tag marshaling: Uses reflection to parse jsonapi struct tags and
//     automatically build the Resource.
//
// Struct Tag Format:
//
//   - Primary key: `jsonapi:"primary,resourcename"`
//     Sets the resource type (pluralized) and ID from the field value.
//
//   - Attributes: `jsonapi:"attr,attributename[,omitempty]"`
//     Maps struct fields to resource attributes. Nested structs are automatically
//     converted to map[string]interface{} using JSON marshaling. The omitempty
//     option skips zero-value fields.
//
//   - Relationships: `jsonapi:"relation,relationname[,omitempty]"`
//     Creates relationships with resource references. For slice fields, marshals
//     each element into a resource reference. For single fields, creates a single
//     resource reference or null if empty. The omitempty option skips nil pointer fields.
//
// Example usage:
//
//	type Address struct {
//	    Street string `json:"street"`
//	    City   string `json:"city"`
//	}
//
//	type User struct {
//	    ID      string  `jsonapi:"primary,user"`
//	    Name    string  `jsonapi:"attr,name"`
//	    Address Address `jsonapi:"attr,address"`
//	    Posts   []Post  `jsonapi:"relation,posts"`
//	}
//
//	type Post struct {
//	    ID    string `jsonapi:"primary,post"`
//	    Title string `jsonapi:"attr,title"`
//	}
//
//	user := &User{
//	    ID: "1",
//	    Name: "John",
//	    Address: Address{Street: "123 Main St", City: "NYC"},
//	    Posts: []Post{{ID: "1", Title: "Hello"}},
//	}
//	resource, err := MarshalResourceWithContext(ctx, user)
//	// Returns: &Resource{
//	//   Type: "users",
//	//   ID: "1",
//	//   Attributes: {
//	//     "name": "John",
//	//     "address": {"street": "123 Main St", "city": "NYC"}
//	//   },
//	//   Relationships: {"posts": {Data: MultiResource(Resource{Type: "posts", ID: "1"})}},
//	// }
//
// Parameters:
//   - out: The struct or pointer to struct to marshal. Must not be nil if pointer.
//
// Returns:
//   - *Resource: Pointer to the marshaled JSON:API resource
//   - error: Error if marshaling fails (nil pointer, unsupported type, etc.)
func MarshalResourceWithContext(ctx context.Context, out interface{}, opts ...func(*MarshalOptions)) (res Resource, err error) {
	options := MarshalOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	if marshaler, ok := out.(ResourceMarshaler); ok {
		res, err = marshaler.MarshalJSONAPIResource(ctx)
	} else {
		res, err = marshalResourceFromStruct(ctx, out, options)
	}

	if err != nil {
		return res, err
	}

	if marshaler, ok := out.(LinksMarshaler); ok {
		links, linkserr := marshaler.MarshalJSONAPILinks(ctx)
		if linkserr == nil {
			res.Links = links
		}
		err = linkserr
	}

	if marshaler, ok := out.(MetaMarshaler); ok {
		meta, metaerr := marshaler.MarshalJSONAPIMeta(ctx)
		if metaerr == nil {
			res.Meta = meta
		}
		err = metaerr
	}

	relerrs := make([]error, 0, len(res.Relationships))
	for name, rel := range res.Relationships {
		if marshaler, ok := out.(RelationshipLinksMarshaler); ok {
			links, linkserr := marshaler.MarshalJSONAPIRelationshipLinks(ctx, name)
			if linkserr == nil {
				rel.Links = links
			}
			relerrs = append(relerrs, linkserr)
		}
		if marshaler, ok := out.(RelationshipMetaMarshaler); ok {
			meta, metaerr := marshaler.MarshalJSONAPIRelationshipMeta(ctx, name)
			if metaerr == nil {
				rel.Meta = meta
			}
			relerrs = append(relerrs, metaerr)
		}
	}

	return res, nil
}

func MarshalResource(out interface{}, opts ...func(*MarshalOptions)) (Resource, error) {
	return MarshalResourceWithContext(context.Background(), out, opts...)
}

// marshalResourceFromStruct performs the actual struct-to-Resource conversion
// using reflection and struct tag parsing. This is the core implementation
// that handles the jsonapi struct tags.
//
// The function processes each exported field of the struct, looking for
// jsonapi tags and building the appropriate Resource components:
//   - Primary tags set the resource type and ID
//   - Attr tags populate the Attributes map
//   - Relation tags create entries in the Relationships map
//
// Parameters:
//   - out: The struct or pointer to struct to process
//
// Returns:
//   - *Resource: The constructed JSON:API resource
//   - error: Error if the input is invalid (nil pointer, non-struct, etc.)
func marshalResourceFromStruct(ctx context.Context, out interface{}, opts MarshalOptions) (res Resource, err error) {
	v := reflect.ValueOf(out)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return res, errors.New("cannot marshal nil pointer")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return res, errors.New("expected struct or pointer to struct")
	}

	t := v.Type()
	res = Resource{
		Attributes:    make(map[string]interface{}),
		Relationships: make(map[string]Relationship),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Handle embedded structs
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			// Process embedded struct fields directly
			embeddedType := fieldValue.Type()
			for j := 0; j < embeddedType.NumField(); j++ {
				embeddedField := embeddedType.Field(j)
				embeddedFieldValue := fieldValue.Field(j)

				// Skip unexported fields
				if !embeddedField.IsExported() {
					continue
				}

				embeddedTag := embeddedField.Tag.Get("jsonapi")
				if embeddedTag == "" {
					continue
				}

				embeddedParts := strings.Split(embeddedTag, ",")
				if len(embeddedParts) < 2 {
					continue
				}

				embeddedTagType := embeddedParts[0]
				embeddedTagName := embeddedParts[1]

				switch embeddedTagType {
				case "primary":
					res.Type = pluralize(embeddedTagName)
					res.ID = embeddedFieldValue.String()

				case "attr":
					if opts.isRef {
						continue
					}

					if !embeddedFieldValue.IsZero() || !hasOmitEmpty(embeddedParts) {
						attrValue, err := marshalAttributeValue(embeddedFieldValue)
						if err != nil {
							return res, err
						}
						res.Attributes[embeddedTagName] = attrValue
					}

				case "relation":
					if opts.isRef {
						continue
					}

					// Handle omitempty for zero values
					if hasOmitEmpty(embeddedParts) && embeddedFieldValue.IsZero() {
						continue
					}

					if embeddedFieldValue.Kind() == reflect.Slice {
						// Handle slice relationships
						var relatedResources []Resource

						// If slice is not empty, marshal each element
						if !embeddedFieldValue.IsZero() && embeddedFieldValue.Len() > 0 {
							for k := 0; k < embeddedFieldValue.Len(); k++ {
								elem := embeddedFieldValue.Index(k)
								relatedResource, err := marshalResourceReference(ctx, elem.Interface())
								if err != nil {
									return res, err
								}
								relatedResources = append(relatedResources, relatedResource)
							}
						}

						res.Relationships[embeddedTagName] = Relationship{
							Data: MultiResource(relatedResources...),
						}
					} else {
						// Handle single relationships
						if embeddedFieldValue.IsZero() {
							res.Relationships[embeddedTagName] = Relationship{
								Data: NullResource(),
							}
						} else {
							relatedResource, err := marshalResourceReference(ctx, embeddedFieldValue.Interface())
							if err != nil {
								return res, err
							}
							res.Relationships[embeddedTagName] = Relationship{
								Data: SingleResource(relatedResource),
							}
						}
					}
				}
			}
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
		tagName := parts[1]

		switch tagType {
		case "primary":
			res.Type = pluralize(tagName)
			res.ID = fieldValue.String()

		case "attr":
			if opts.isRef {
				continue
			}

			if !fieldValue.IsZero() || !hasOmitEmpty(parts) {
				attrValue, err := marshalAttributeValue(fieldValue)
				if err != nil {
					return res, err
				}
				res.Attributes[tagName] = attrValue
			}

		case "relation":
			if opts.isRef {
				continue
			}

			// Handle omitempty for zero values
			if hasOmitEmpty(parts) && fieldValue.IsZero() {
				continue
			}

			if fieldValue.Kind() == reflect.Slice {
				// Handle slice relationships
				var relatedResources []Resource

				// If slice is not empty, marshal each element
				if !fieldValue.IsZero() && fieldValue.Len() > 0 {
					for j := 0; j < fieldValue.Len(); j++ {
						elem := fieldValue.Index(j)
						relatedResource, err := marshalResourceReference(ctx, elem.Interface())
						if err != nil {
							return res, err
						}
						relatedResources = append(relatedResources, relatedResource)
					}
				}

				res.Relationships[tagName] = Relationship{
					Data: MultiResource(relatedResources...),
				}
			} else {
				// Handle single relationships
				if fieldValue.IsZero() {
					res.Relationships[tagName] = Relationship{
						Data: NullResource(),
					}
				} else {
					relatedResource, err := marshalResourceReference(ctx, fieldValue.Interface())
					if err != nil {
						return res, err
					}
					res.Relationships[tagName] = Relationship{
						Data: SingleResource(relatedResource),
					}
				}
			}
		}
	}

	return res, nil
}

// marshalAttributeValue converts a field value to an appropriate attribute value.
// For structs, it converts them to map[string]interface{} using JSON marshaling/unmarshaling.
// For other types, it returns the value as-is.
//
// Parameters:
//   - fieldValue: The reflect.Value of the field to marshal
//
// Returns:
//   - interface{}: The marshaled attribute value
//   - error: Error if marshaling fails
func marshalAttributeValue(fieldValue reflect.Value) (interface{}, error) {
	fieldInterface := fieldValue.Interface()

	// Special case for time.Time - treat it as a primitive value
	if _, ok := fieldInterface.(time.Time); ok {
		return fieldInterface, nil
	}

	// If it's a struct (but not a pointer), convert it to map[string]interface{}
	if fieldValue.Kind() == reflect.Struct {
		return structToMap(fieldInterface)
	}

	// For all other types, return as-is
	return fieldInterface, nil
}

// structToMap converts a struct to map[string]interface{} using JSON marshaling.
// This ensures that nested structs are properly serialized for JSON:API attributes.
//
// Parameters:
//   - s: The struct to convert
//
// Returns:
//   - map[string]interface{}: The struct converted to a map
//   - error: Error if conversion fails
func structToMap(s interface{}) (map[string]interface{}, error) {
	// Use JSON marshaling to convert struct to map
	// This handles all the JSON tags and nested structures properly

	// Marshal to JSON
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	// Unmarshal back to map
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// marshalResourceReference creates a resource reference (ID and Type only) from a struct.
// This is used for relationships where we only need the resource identifier, not the full data.
//
// Parameters:
//   - out: The struct or pointer to struct to create a reference for
//
// Returns:
//   - Resource: A resource with only ID and Type populated (reference)
//   - error: Error if marshaling fails
func marshalResourceReference(ctx context.Context, out interface{}) (Resource, error) {
	fullResource, err := MarshalResourceWithContext(ctx, out, func(mo *MarshalOptions) { mo.isRef = true })
	if err != nil {
		return Resource{}, err
	}

	// Return only the reference (ID and Type)
	return fullResource.Ref(), nil
}

// hasOmitEmpty checks if the "omitempty" option is present in the struct tag parts.
// This is used to determine whether zero-value fields should be omitted from
// the marshaled output.
//
// Parameters:
//   - parts: Slice of tag components split by comma
//
// Returns:
//   - bool: true if "omitempty" is found in the parts, false otherwise
func hasOmitEmpty(parts []string) bool {
	for _, part := range parts {
		if part == "omitempty" {
			return true
		}
	}
	return false
}

// pluralize converts a singular resource name to its plural form for use as
// the JSON:API resource type. Currently implements simple pluralization by
// appending 's' to the input word.
//
// This is a basic implementation that could be enhanced with more sophisticated
// pluralization rules (e.g., handling irregular plurals like "person" -> "people").
//
// Parameters:
//   - word: The singular resource name to pluralize
//
// Returns:
//   - string: The pluralized resource type name
//
// Example:
//   - pluralize("user") returns "users"
//   - pluralize("order") returns "orders"
func pluralize(word string) string {
	// Simple pluralization - add 's' to the end
	// This could be made more sophisticated with proper pluralization rules
	return word + "s"
}
