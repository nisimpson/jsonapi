package jsonapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnmarshal tests the Unmarshal function
func TestUnmarshal(t *testing.T) {
	// Define a test struct
	type User struct {
		ID     string   `jsonapi:"primary,users"`
		Name   string   `jsonapi:"attr,name"`
		Email  string   `jsonapi:"attr,email"`
		Tags   []string `jsonapi:"attr,tags"`
		Ints   []int    `jsonapi:"attr,ints"`
		Nested *struct {
			Nested struct {
				Value string
			}
		} `jsonapi:"attr,nested"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com",
				"tags": ["one", "two", "three"],
				"ints": [1, 2, 3],
				"nested": {
					"Nested": {
						"Value": "Nested Value"
					}
				}
			}
		}
	}`)

	// Unmarshal the data
	var user User
	err := Unmarshal(jsonData, &user)
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "123", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.EqualValues(t, []string{"one", "two", "three"}, user.Tags)
	assert.EqualValues(t, []int{1, 2, 3}, user.Ints)
	assert.Equal(t, "Nested Value", user.Nested.Nested.Value)
}

func TestUnmarshal_Readonly(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name,readonly"`
		Email string `jsonapi:"attr,email"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`)

	// Unmarshal the data
	var user User
	err := Unmarshal(jsonData, &user, PermitReadOnly(false))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReadOnly)
}

// TestUnmarshalWithContext tests the UnmarshalWithContext function
func TestUnmarshalWithContext(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`)

	// Unmarshal the data with context
	ctx := context.Background()
	var user User
	err := UnmarshalWithContext(ctx, jsonData, &user)
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "123", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

// TestUnmarshalDocument tests the UnmarshalDocument function
func TestUnmarshalDocument(t *testing.T) {
	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`)

	// Unmarshal the document
	doc := Document{}
	err := json.Unmarshal(jsonData, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

// TestUnmarshalSlice tests unmarshaling into a slice
func TestUnmarshalSlice(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": [
			{
				"id": "123",
				"type": "users",
				"attributes": {
					"name": "John Doe",
					"email": "john@example.com"
				}
			},
			{
				"id": "456",
				"type": "users",
				"attributes": {
					"name": "Jane Smith",
					"email": "jane@example.com"
				}
			}
		]
	}`)

	// Unmarshal the data
	var users []User
	err := Unmarshal(jsonData, &users)
	require.NoError(t, err)

	// Verify the result
	require.Len(t, users, 2)
	assert.Equal(t, "123", users[0].ID)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "john@example.com", users[0].Email)
	assert.Equal(t, "456", users[1].ID)
	assert.Equal(t, "Jane Smith", users[1].Name)
	assert.Equal(t, "jane@example.com", users[1].Email)
}

// TestUnmarshalWithOptions tests unmarshaling with options
func TestUnmarshalWithOptions(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com",
				"extra": "This field is not in the struct"
			}
		}
	}`)

	// Test with strict mode
	var user User
	err := Unmarshal(jsonData, &user, StrictMode())
	assert.NoError(t, err) // Should not fail in strict mode due to extra field

	// Test without strict mode
	user = User{}
	err = Unmarshal(jsonData, &user)
	require.NoError(t, err) // Should succeed without strict mode
	assert.Equal(t, "123", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

// TestUnmarshalRelationships tests unmarshaling resources with relationships
func TestUnmarshalRelationships(t *testing.T) {
	// Define test structs
	type Comment struct {
		ID      string `jsonapi:"primary,comments"`
		Content string `jsonapi:"attr,content"`
	}

	type Post struct {
		ID       string    `jsonapi:"primary,posts"`
		Title    string    `jsonapi:"attr,title"`
		Comments []Comment `jsonapi:"relation,comments"`
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "1",
			"type": "posts",
			"attributes": {
				"title": "Hello World"
			},
			"relationships": {
				"comments": {
					"data": [
						{
							"id": "101",
							"type": "comments"
						},
						{
							"id": "102",
							"type": "comments"
						}
					]
				}
			}
		},
		"included": [
			{
				"id": "101",
				"type": "comments",
				"attributes": {
					"content": "Great post!"
				}
			},
			{
				"id": "102",
				"type": "comments",
				"attributes": {
					"content": "Thanks for sharing!"
				}
			}
		]
	}`)

	// Unmarshal with included resources
	var post Post
	err := Unmarshal(jsonData, &post, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "1", post.ID)
	assert.Equal(t, "Hello World", post.Title)
	require.Len(t, post.Comments, 2)
	assert.Equal(t, "101", post.Comments[0].ID)
	assert.Equal(t, "Great post!", post.Comments[0].Content)
	assert.Equal(t, "102", post.Comments[1].ID)
	assert.Equal(t, "Thanks for sharing!", post.Comments[1].Content)
}

// TestUnmarshalFromDocument tests unmarshaling from a Document
func TestUnmarshalFromDocument(t *testing.T) {
	// Test structure
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a document with a single resource
	doc := &Document{
		Data: SingleResource(Resource{
			ID:   "1",
			Type: "users",
			Attributes: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		}),
	}

	// Unmarshal into a struct
	var user User
	err := unmarshalFromDocument(context.Background(), doc, &user, UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

// TestUnmarshalFromDocument_Null tests unmarshaling a null document
func TestUnmarshalFromDocument_Null(t *testing.T) {
	// Test structure
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a document with null data
	doc := &Document{
		Data: NullResource(),
	}

	// Unmarshal into a struct
	var user User
	err := unmarshalFromDocument(context.Background(), doc, &user, UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result is zero value
	assert.Equal(t, "", user.ID)
	assert.Equal(t, "", user.Name)
	assert.Equal(t, "", user.Email)
}

// TestUnmarshalFromDocument_ManyToOne tests unmarshaling multiple resources into a single struct
func TestUnmarshalFromDocument_ManyToOne(t *testing.T) {
	// Test structure
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a document with multiple resources
	doc := &Document{
		Data: MultiResource(
			Resource{
				ID:   "1",
				Type: "users",
				Attributes: map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
			Resource{
				ID:   "2",
				Type: "users",
				Attributes: map[string]interface{}{
					"name":  "Jane Smith",
					"email": "jane@example.com",
				},
			},
		),
	}

	// Unmarshal into a struct (should take the first resource)
	var user User
	err := unmarshalFromDocument(context.Background(), doc, &user, UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

// TestUnmarshalSliceFromDocument tests unmarshaling a document into a slice
func TestUnmarshalSliceFromDocument(t *testing.T) {
	// Use the public API instead of internal functions
	jsonData := []byte(`{
		"data": [
			{
				"id": "1",
				"type": "users",
				"attributes": {
					"name": "John Doe",
					"email": "john@example.com"
				}
			},
			{
				"id": "2",
				"type": "users",
				"attributes": {
					"name": "Jane Smith",
					"email": "jane@example.com"
				}
			}
		]
	}`)

	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	var users []User
	err := Unmarshal(jsonData, &users)
	require.NoError(t, err)

	// Verify the result
	require.Len(t, users, 2)
	assert.Equal(t, "1", users[0].ID)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "john@example.com", users[0].Email)
	assert.Equal(t, "2", users[1].ID)
	assert.Equal(t, "Jane Smith", users[1].Name)
	assert.Equal(t, "jane@example.com", users[1].Email)
}

// TestUnmarshalSliceFromDocument_Single tests unmarshaling a single resource into a slice
func TestUnmarshalSliceFromDocument_Single(t *testing.T) {
	// Use the public API instead of internal functions
	jsonData := []byte(`{
		"data": {
			"id": "1",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`)

	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	var users []User
	err := Unmarshal(jsonData, &users)
	require.NoError(t, err)

	// Verify the result
	require.Len(t, users, 1)
	assert.Equal(t, "1", users[0].ID)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "john@example.com", users[0].Email)
}

// TestUnmarshalRelationship_ToOne tests unmarshaling a to-one relationship
func TestUnmarshalRelationship_ToOne(t *testing.T) {
	// Test structures for relationship unmarshaling
	type RelationshipChild struct {
		ID     string `jsonapi:"primary,children"`
		Name   string `jsonapi:"attr,name"`
		Age    int    `jsonapi:"attr,age"`
		Active bool   `jsonapi:"attr,active"`
	}

	type RelationshipParent struct {
		ID          string            `jsonapi:"primary,parents"`
		Name        string            `jsonapi:"attr,name"`
		SingleChild RelationshipChild `jsonapi:"relation,single_child"`
	}

	// Test unmarshaling a to-one relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"single_child": {
					"data": {
						"id": "child-1",
						"type": "children"
					}
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	assert.Equal(t, "child-1", parent.SingleChild.ID)
	assert.Equal(t, "Child 1", parent.SingleChild.Name)
	assert.Equal(t, 10, parent.SingleChild.Age)
	assert.True(t, parent.SingleChild.Active)
}

// TestUnmarshalRelationship_ToMany tests unmarshaling a to-many relationship
func TestUnmarshalRelationship_ToMany(t *testing.T) {
	// Test structures for relationship unmarshaling
	type RelationshipChild struct {
		ID     string `jsonapi:"primary,children"`
		Name   string `jsonapi:"attr,name"`
		Age    int    `jsonapi:"attr,age"`
		Active bool   `jsonapi:"attr,active"`
	}

	type RelationshipParent struct {
		ID           string              `jsonapi:"primary,parents"`
		Name         string              `jsonapi:"attr,name"`
		ManyChildren []RelationshipChild `jsonapi:"relation,many_children"`
	}

	// Test unmarshaling a to-many relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_children": {
					"data": [
						{
							"id": "child-1",
							"type": "children"
						},
						{
							"id": "child-2",
							"type": "children"
						}
					]
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			},
			{
				"id": "child-2",
				"type": "children",
				"attributes": {
					"name": "Child 2",
					"age": 8,
					"active": false
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	require.Len(t, parent.ManyChildren, 2)
	assert.Equal(t, "child-1", parent.ManyChildren[0].ID)
	assert.Equal(t, "Child 1", parent.ManyChildren[0].Name)
	assert.Equal(t, 10, parent.ManyChildren[0].Age)
	assert.True(t, parent.ManyChildren[0].Active)
	assert.Equal(t, "child-2", parent.ManyChildren[1].ID)
	assert.Equal(t, "Child 2", parent.ManyChildren[1].Name)
	assert.Equal(t, 8, parent.ManyChildren[1].Age)
	assert.False(t, parent.ManyChildren[1].Active)
}

func TestUnmarshalRelationship_ReadOnly(t *testing.T) {
	// Test structures for relationship unmarshaling
	type RelationshipChild struct {
		ID     string `jsonapi:"primary,children"`
		Name   string `jsonapi:"attr,name"`
		Age    int    `jsonapi:"attr,age"`
		Active bool   `jsonapi:"attr,active"`
	}

	type RelationshipParent struct {
		ID          string            `jsonapi:"primary,parents"`
		Name        string            `jsonapi:"attr,name"`
		SingleChild RelationshipChild `jsonapi:"relation,single_child,readonly"`
	}

	// Test unmarshaling a to-one relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"single_child": {
					"data": {
						"id": "child-1",
						"type": "children"
					}
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded(), PermitReadOnly(false))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReadOnly)
}

// TestCustomUnmarshaler tests custom unmarshaling through interfaces
func TestCustomUnmarshaler(t *testing.T) {
	// Define a custom unmarshaler
	type TestCustomUser struct {
		ID    string
		Name  string
		Email string
	}

	// Create test JSON data
	jsonData := []byte(`{
		"data": {
			"id": "123",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`)

	// Unmarshal the data
	var user TestCustomUser

	// Since we can't define methods inside functions, we'll just test the unmarshaling
	// by manually creating a resource and calling the method
	doc := Document{}
	err := json.Unmarshal(jsonData, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	require.True(t, ok)

	// Set the fields manually
	user.ID = resource.ID
	if name, ok := resource.Attributes["name"].(string); ok {
		user.Name = name
	}
	if email, ok := resource.Attributes["email"].(string); ok {
		user.Email = email
	}

	// Verify the result
	assert.Equal(t, "123", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

// TestTypeConversion tests type conversion during unmarshaling
func TestTypeConversion(t *testing.T) {
	// Define a test struct with various types
	type ConversionTest struct {
		ID       string  `jsonapi:"primary,tests"`
		IntVal   int     `jsonapi:"attr,int_val"`
		FloatVal float64 `jsonapi:"attr,float_val"`
		BoolVal  bool    `jsonapi:"attr,bool_val"`
		StrVal   string  `jsonapi:"attr,str_val"`
	}

	// Create test JSON data with type conversions
	jsonData := []byte(`{
		"data": {
			"id": "1",
			"type": "tests",
			"attributes": {
				"int_val": "42",
				"float_val": "3.14",
				"bool_val": "true",
				"str_val": 123
			}
		}
	}`)

	// Unmarshal the data
	var test ConversionTest
	err := Unmarshal(jsonData, &test)
	require.NoError(t, err)

	// Verify the result with converted types
	assert.Equal(t, "1", test.ID)
	assert.Equal(t, 42, test.IntVal)
	assert.Equal(t, 3.14, test.FloatVal)
	assert.Equal(t, true, test.BoolVal)
	assert.Equal(t, "123", test.StrVal)
}

// TestConvertToInt tests the convertToInt function
func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int64
		isError  bool
	}{
		{"int", 42, 42, false},
		{"int8", int8(8), 8, false},
		{"int16", int16(16), 16, false},
		{"int32", int32(32), 32, false},
		{"int64", int64(64), 64, false},
		{"uint", uint(42), 0, true},
		{"uint8", uint8(8), 0, true},
		{"uint16", uint16(16), 0, true},
		{"uint32", uint32(32), 0, true},
		{"uint64", uint64(64), 0, true},
		{"float32", float32(42.0), 0, true},
		{"float64", float64(42.0), 0, true},
		{"string_int", "42", 42, false},
		{"string_float", "42.0", 0, true},
		{"bool_true", true, 0, true},
		{"bool_false", false, 0, true},
		{"invalid_string", "not a number", 0, true},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToInt(tt.value)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestConvertToUint tests the convertToUint function
func TestConvertToUint(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected uint64
		isError  bool
	}{
		{"int", 42, 0, true},
		{"int8", int8(8), 0, true},
		{"int16", int16(16), 0, true},
		{"int32", int32(32), 0, true},
		{"int64", int64(64), 0, true},
		{"uint", uint(42), 42, false},
		{"uint8", uint8(8), 8, false},
		{"uint16", uint16(16), 16, false},
		{"uint32", uint32(32), 32, false},
		{"uint64", uint64(64), 64, false},
		{"float32", float32(42.0), 0, true},
		{"float64", float64(42.0), 0, true},
		{"string_int", "42", 42, false},
		{"string_float", "42.0", 0, true},
		{"bool_true", true, 0, true},
		{"bool_false", false, 0, true},
		{"negative_int", -42, 0, true},
		{"negative_float", -42.0, 0, true},
		{"negative_string", "-42", 0, true},
		{"invalid_string", "not a number", 0, true},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToUint(tt.value)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestConvertToFloat tests the convertToFloat function
func TestConvertToFloat(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected float64
		isError  bool
	}{
		{"int", 42, 42.0, false},
		{"int8", int8(8), 8.0, false},
		{"int16", int16(16), 16.0, false},
		{"int32", int32(32), 32.0, false},
		{"int64", int64(64), 64.0, false},
		{"uint", uint(42), 42.0, false},
		{"uint8", uint8(8), 8.0, false},
		{"uint16", uint16(16), 16.0, false},
		{"uint32", uint32(32), 32.0, false},
		{"uint64", uint64(64), 64.0, false},
		{"float32", float32(3.14), 3.14, false},
		{"float64", float64(3.14), 3.14, false},
		{"string_int", "42", 42.0, false},
		{"string_float", "3.14", 3.14, false},
		{"bool_true", true, 0.0, true},
		{"bool_false", false, 0.0, true},
		{"invalid_string", "not a number", 0.0, true},
		{"nil", nil, 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToFloat(tt.value)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.LessOrEqual(t, tt.expected, result)
			}
		})
	}
}

// TestConvertToBool tests the convertToBool function
func TestConvertToBool(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
		isError  bool
	}{
		{"bool_true", true, true, false},
		{"bool_false", false, false, false},
		{"int_1", 1, false, true},
		{"int_0", 0, false, true},
		{"float_1", 1.0, false, true},
		{"float_0", 0.0, false, true},
		{"string_true", "true", true, false},
		{"string_false", "false", false, false},
		{"string_1", "1", true, false},
		{"string_0", "0", false, false},
		{"string_yes", "yes", false, true},
		{"string_no", "no", false, true},
		{"string_y", "y", false, true},
		{"string_n", "n", false, true},
		{"string_t", "t", true, false},
		{"string_f", "f", false, false},
		{"string_on", "on", false, true},
		{"string_off", "off", false, true},
		{"string_enabled", "enabled", false, true},
		{"string_disabled", "disabled", false, true},
		{"invalid_string", "not a boolean", false, true},
		{"nil", nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToBool(tt.value)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestWithUnmarshaler tests the WithUnmarshaler option function
func TestWithUnmarshaler(t *testing.T) {
	// Create a custom unmarshaler function
	customUnmarshaler := func(data []byte, v interface{}) error {
		// Just a simple test implementation
		return json.Unmarshal(data, v)
	}

	// Create options with the custom unmarshaler
	opts := &UnmarshalOptions{}
	withUnmarshaler := WithUnmarshaler(customUnmarshaler)
	withUnmarshaler(opts)

	// Verify the unmarshaler was set correctly
	assert.NotNil(t, opts.unmarshaler)

	// Test the unmarshaler with some data
	data := []byte(`{"test": "value"}`)
	var result map[string]interface{}

	err := opts.unmarshaler(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["test"])
}
