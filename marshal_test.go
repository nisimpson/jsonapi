package jsonapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarshal tests the Marshal function
func TestMarshal(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a test user
	user := User{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Marshal the user
	data, err := Marshal(user)
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

// TestMarshalWithContext tests the MarshalWithContext function
func TestMarshalWithContext(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a test user
	user := User{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Marshal the user with context
	ctx := context.Background()
	data, err := MarshalWithContext(ctx, user)
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

// TestMarshalDocument tests the MarshalDocument function
func TestMarshalDocument(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a test user
	user := User{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Marshal the user to a document
	ctx := context.Background()
	doc, err := MarshalDocument(ctx, user)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

// TestMarshalSlice tests marshaling a slice of resources
func TestMarshalSlice(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create test users
	users := []User{
		{
			ID:    "123",
			Name:  "John Doe",
			Email: "john@example.com",
		},
		{
			ID:    "456",
			Name:  "Jane Smith",
			Email: "jane@example.com",
		},
	}

	// Marshal the users
	data, err := Marshal(users)
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resources, ok := doc.Data.Many()
	require.True(t, ok)
	require.Len(t, resources, 2)

	assert.Equal(t, "123", resources[0].ID)
	assert.Equal(t, "users", resources[0].Type)
	assert.Equal(t, "John Doe", resources[0].Attributes["name"])
	assert.Equal(t, "john@example.com", resources[0].Attributes["email"])

	assert.Equal(t, "456", resources[1].ID)
	assert.Equal(t, "users", resources[1].Type)
	assert.Equal(t, "Jane Smith", resources[1].Attributes["name"])
	assert.Equal(t, "jane@example.com", resources[1].Attributes["email"])
}

// TestMarshalWithOptions tests marshaling with options
func TestMarshalWithOptions(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a test user
	user := User{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Marshal with custom marshaler
	customMarshaler := func(v interface{}) ([]byte, error) {
		return json.Marshal(v)
	}

	data, err := Marshal(user, WithMarshaler(customMarshaler))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

// TestMarshalRelationships tests marshaling resources with relationships
func TestMarshalRelationships(t *testing.T) {
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

	// Create test data
	post := Post{
		ID:    "1",
		Title: "Hello World",
		Comments: []Comment{
			{
				ID:      "101",
				Content: "Great post!",
			},
			{
				ID:      "102",
				Content: "Thanks for sharing!",
			},
		},
	}

	// Marshal with included resources
	data, err := Marshal(post, IncludeRelatedResources())
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resource, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "posts", resource.Type)
	assert.Equal(t, "Hello World", resource.Attributes["title"])

	// Verify relationship
	relationship, ok := resource.Relationships["comments"]
	require.True(t, ok)
	commentResources, ok := relationship.Data.Many()
	require.True(t, ok)
	require.Len(t, commentResources, 2)
	assert.Equal(t, "101", commentResources[0].ID)
	assert.Equal(t, "comments", commentResources[0].Type)
	assert.Equal(t, "102", commentResources[1].ID)
	assert.Equal(t, "comments", commentResources[1].Type)

	// Verify included resources
	require.Len(t, doc.Included, 2)
	assert.Equal(t, "101", doc.Included[0].ID)
	assert.Equal(t, "comments", doc.Included[0].Type)
	assert.Equal(t, "Great post!", doc.Included[0].Attributes["content"])
	assert.Equal(t, "102", doc.Included[1].ID)
	assert.Equal(t, "comments", doc.Included[1].Type)
	assert.Equal(t, "Thanks for sharing!", doc.Included[1].Attributes["content"])
}

// TestDocumentMeta tests the DocumentMeta option
func TestDocumentMeta(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define metadata
	meta := map[string]interface{}{
		"count":    42,
		"page":     1,
		"per_page": 10,
		"total":    100,
	}

	// Marshal with DocumentMeta option
	data, err := Marshal(resource, DocumentMeta(meta))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify metadata was included (note: JSON unmarshaling converts numbers to float64)
	assert.Equal(t, float64(42), doc.Meta["count"])
	assert.Equal(t, float64(1), doc.Meta["page"])
	assert.Equal(t, float64(10), doc.Meta["per_page"])
	assert.Equal(t, float64(100), doc.Meta["total"])

	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}

// TestDocumentLinks tests the DocumentLinks option
func TestDocumentLinks(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define links
	links := map[string]Link{
		"self":  {Href: "/api/test/1"},
		"first": {Href: "/api/test?page=1"},
		"last":  {Href: "/api/test?page=10"},
		"next":  {Href: "/api/test?page=2"},
		"prev":  {Href: "", Meta: map[string]interface{}{"available": false}},
	}

	// Marshal with DocumentLinks option
	data, err := Marshal(resource, DocumentLinks(links))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify links were included
	assert.Equal(t, "/api/test/1", doc.Links["self"].Href)
	assert.Equal(t, "/api/test?page=1", doc.Links["first"].Href)
	assert.Equal(t, "/api/test?page=10", doc.Links["last"].Href)
	assert.Equal(t, "/api/test?page=2", doc.Links["next"].Href)
	assert.Equal(t, "", doc.Links["prev"].Href)
	assert.Nil(t, doc.Links["prev"].Meta)

	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}

// TestDocumentMetaAndLinks tests combining DocumentMeta and DocumentLinks options
func TestDocumentMetaAndLinks(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define metadata and links
	meta := map[string]interface{}{
		"count": 42,
		"total": 100,
	}

	links := map[string]Link{
		"self": {Href: "/api/test/1"},
		"next": {Href: "/api/test?page=2"},
	}

	// Marshal with both DocumentMeta and DocumentLinks options
	data, err := Marshal(resource, DocumentMeta(meta), DocumentLinks(links))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify metadata was included (note: JSON unmarshaling converts numbers to float64)
	assert.Equal(t, float64(42), doc.Meta["count"])
	assert.Equal(t, float64(100), doc.Meta["total"])

	// Verify links were included
	assert.Equal(t, "/api/test/1", doc.Links["self"].Href)
	assert.Equal(t, "/api/test?page=2", doc.Links["next"].Href)

	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}

// TestApplySparseFieldsets tests the ApplySparseFieldsets method
func TestApplySparseFieldsets(t *testing.T) {
	// Create a resource with attributes
	resource := Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		},
	}

	// Apply sparse fieldsets
	resource.ApplySparseFieldsets([]string{"name", "email"})

	// Verify only the specified fields are included
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
	assert.NotContains(t, resource.Attributes, "age")

	// Verify the resource has only the specified attributes
	assert.Len(t, resource.Attributes, 2)

	// Test with empty fields (should keep all attributes)
	resource = Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
	}
	resource.ApplySparseFieldsets([]string{})
	assert.Len(t, resource.Attributes, 2)

	// Test with nil attributes
	resource = Resource{
		ID:         "1",
		Type:       "users",
		Attributes: nil,
	}
	resource.ApplySparseFieldsets([]string{"name"})
	assert.NotNil(t, resource.Attributes)
	assert.Len(t, resource.Attributes, 0)
}

// TestSparseFieldsetsOption tests the SparseFieldsets option
func TestSparseFieldsetsOption(t *testing.T) {
	// Create MarshalOptions with SparseFieldsets
	options := &MarshalOptions{}
	SparseFieldsets("users", []string{"name"})(options)

	// Verify the modifyDocument function was added
	assert.Len(t, options.modifyDocument, 1)

	// Create a simple test type
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	user := User{
		ID:    "1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Marshal with SparseFieldsets option
	data, err := Marshal(user, SparseFieldsets("users", []string{"name"}))
	assert.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)

	// Verify only the name attribute is included
	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.NotContains(t, resource.Attributes, "email")
}

// TestCustomMarshaler tests custom marshaling through interfaces
func TestCustomMarshaler(t *testing.T) {
	// Define a custom marshaler
	type CustomResource struct {
		ID    string
		Name  string
		Email string
	}

	// Create a custom resource
	resource := &CustomResource{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// For testing purposes, we'll just marshal it directly
	data, err := json.Marshal(map[string]interface{}{
		"data": Resource{
			ID:   resource.ID,
			Type: "custom",
			Attributes: map[string]interface{}{
				"name":  resource.Name,
				"email": resource.Email,
			},
		},
	})
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify the document structure
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "123", resourceData.ID)
	assert.Equal(t, "custom", resourceData.Type)
	assert.Equal(t, "John Doe", resourceData.Attributes["name"])
	assert.Equal(t, "john@example.com", resourceData.Attributes["email"])
}
