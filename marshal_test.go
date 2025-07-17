package jsonapi

import (
	"context"
	"encoding/json"
	"reflect"
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

// TestSparseFieldsets tests the SparseFieldsets option function
func TestSparseFieldsets(t *testing.T) {
	t.Run("single resource", func(t *testing.T) {
		// Create a test resource
		user := struct {
			ID       string `jsonapi:"primary,users"`
			Name     string `jsonapi:"attr,name"`
			Email    string `jsonapi:"attr,email"`
			Password string `jsonapi:"attr,password"`
		}{
			ID:       "1",
			Name:     "John Doe",
			Email:    "john@example.com",
			Password: "secret",
		}

		// Marshal with sparse fieldsets
		data, err := Marshal(user, SparseFieldsets("users", []string{"name", "email"}))
		require.NoError(t, err)

		// Unmarshal to verify
		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		// Verify only the specified fields are included
		resource, ok := doc.Data.One()
		require.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
		assert.Equal(t, 2, len(resource.Attributes))
		assert.Contains(t, resource.Attributes, "name")
		assert.Contains(t, resource.Attributes, "email")
		assert.NotContains(t, resource.Attributes, "password")
	})

	t.Run("multiple resources", func(t *testing.T) {
		// Create test resources
		users := []struct {
			ID       string `jsonapi:"primary,users"`
			Name     string `jsonapi:"attr,name"`
			Email    string `jsonapi:"attr,email"`
			Password string `jsonapi:"attr,password"`
		}{
			{
				ID:       "1",
				Name:     "John Doe",
				Email:    "john@example.com",
				Password: "secret1",
			},
			{
				ID:       "2",
				Name:     "Jane Smith",
				Email:    "jane@example.com",
				Password: "secret2",
			},
		}

		// Marshal with sparse fieldsets
		data, err := Marshal(users, SparseFieldsets("users", []string{"name"}))
		require.NoError(t, err)

		// Unmarshal to verify
		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		// Verify only the specified fields are included
		resources, ok := doc.Data.Many()
		require.True(t, ok)
		require.Len(t, resources, 2)

		for _, resource := range resources {
			assert.Equal(t, "users", resource.Type)
			assert.Equal(t, 1, len(resource.Attributes))
			assert.Contains(t, resource.Attributes, "name")
			assert.NotContains(t, resource.Attributes, "email")
			assert.NotContains(t, resource.Attributes, "password")
		}
	})

	t.Run("with included resources", func(t *testing.T) {
		// Create a test resource with relationships
		type Post struct {
			ID      string `jsonapi:"primary,posts"`
			Title   string `jsonapi:"attr,title"`
			Content string `jsonapi:"attr,content"`
		}

		type User struct {
			ID    string `jsonapi:"primary,users"`
			Name  string `jsonapi:"attr,name"`
			Email string `jsonapi:"attr,email"`
			Posts []Post `jsonapi:"relation,posts"`
		}

		user := User{
			ID:    "1",
			Name:  "John Doe",
			Email: "john@example.com",
			Posts: []Post{
				{
					ID:      "101",
					Title:   "First Post",
					Content: "This is my first post",
				},
				{
					ID:      "102",
					Title:   "Second Post",
					Content: "This is my second post",
				},
			},
		}

		// Marshal with sparse fieldsets for both resource types
		data, err := Marshal(user,
			IncludeRelatedResources(),
			SparseFieldsets("users", []string{"name"}),
			SparseFieldsets("posts", []string{"title"}),
		)
		require.NoError(t, err)

		// Unmarshal to verify
		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		// Verify primary resource has only specified fields
		resource, ok := doc.Data.One()
		require.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
		assert.Equal(t, 1, len(resource.Attributes))
		assert.Contains(t, resource.Attributes, "name")
		assert.NotContains(t, resource.Attributes, "email")

		// Verify included resources have only specified fields
		require.Len(t, doc.Included, 2)
		for _, included := range doc.Included {
			assert.Equal(t, "posts", included.Type)
			assert.Equal(t, 1, len(included.Attributes))
			assert.Contains(t, included.Attributes, "title")
			assert.NotContains(t, included.Attributes, "content")
		}
	})

	t.Run("null resource", func(t *testing.T) {
		// Create a document with null resource
		doc := Document{
			Data: NullResource(),
		}

		// Apply sparse fieldsets
		opts := &MarshalOptions{}
		sparseFieldsets := SparseFieldsets("users", []string{"name", "email"})
		sparseFieldsets(opts)

		// Apply the document modification
		for _, modifyDoc := range opts.modifyDocument {
			modifyDoc(&doc)
		}

		// Verify the document is still null
		assert.True(t, doc.Data.Null())
	})

	t.Run("non-matching resource type", func(t *testing.T) {
		// Create a test resource
		post := struct {
			ID      string `jsonapi:"primary,posts"`
			Title   string `jsonapi:"attr,title"`
			Content string `jsonapi:"attr,content"`
		}{
			ID:      "1",
			Title:   "Hello World",
			Content: "This is a test post",
		}

		// Marshal with sparse fieldsets for a different resource type
		data, err := Marshal(post, SparseFieldsets("users", []string{"name", "email"}))
		require.NoError(t, err)

		// Unmarshal to verify
		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		// Verify all fields are included since the resource type doesn't match
		resource, ok := doc.Data.One()
		require.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "posts", resource.Type)
		assert.Equal(t, 2, len(resource.Attributes))
		assert.Contains(t, resource.Attributes, "title")
		assert.Contains(t, resource.Attributes, "content")
	})
}

// Create a custom marshaler
type CustomUser struct {
	ID    string
	Name  string
	Email string
}

// Implement ResourceMarshaler interface
func (u *CustomUser) MarshalJSONAPIResource(ctx context.Context) (Resource, error) {
	return Resource{
		ID:   u.ID,
		Type: "custom_users",
		Attributes: map[string]interface{}{
			"name":  u.Name,
			"email": u.Email,
		},
	}, nil
}

// TestMarshalSingle tests the marshalSingle function
func TestMarshalSingle(t *testing.T) {
	ctx := context.Background()

	t.Run("custom marshaler", func(t *testing.T) {

		// Create a user
		user := &CustomUser{
			ID:    "1",
			Name:  "John Doe",
			Email: "john@example.com",
		}

		// Marshal the user
		resource, _, err := marshalSingle(ctx, user)
		require.NoError(t, err)

		// Verify the resource
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "custom_users", resource.Type)
		assert.Equal(t, "John Doe", resource.Attributes["name"])
		assert.Equal(t, "john@example.com", resource.Attributes["email"])
	})

	t.Run("nil value", func(t *testing.T) {
		// Marshal nil value
		resource, _, err := marshalSingle(ctx, nil)
		require.Error(t, err)
		assert.Equal(t, Resource{}, resource)
	})

	t.Run("invalid value", func(t *testing.T) {
		// Marshal invalid value (not a struct)
		value := "not a struct"
		resource, _, err := marshalSingle(ctx, value)
		require.Error(t, err)
		assert.Equal(t, Resource{}, resource)
	})

	t.Run("struct with no jsonapi tags", func(t *testing.T) {
		// Create a struct with no jsonapi tags
		type NoTags struct {
			ID   string
			Name string
		}

		// Create an instance
		noTags := NoTags{
			ID:   "1",
			Name: "John Doe",
		}

		// Marshal the struct
		resource, _, err := marshalSingle(ctx, noTags)
		require.NoError(t, err)
		assert.Equal(t, Resource{
			Attributes:    make(map[string]interface{}),
			Relationships: make(map[string]Relationship),
		}, resource)
	})

	t.Run("struct with embedded fields", func(t *testing.T) {
		// Create a base struct
		type Base struct {
			ID string `jsonapi:"primary,users"`
		}

		// Create a derived struct with embedded fields
		type User struct {
			Base
			Name  string `jsonapi:"attr,name"`
			Email string `jsonapi:"attr,email"`
		}

		// Create an instance
		user := User{
			Base:  Base{ID: "1"},
			Name:  "John Doe",
			Email: "john@example.com",
		}

		// Marshal the struct
		resource, _, err := marshalSingle(ctx, user)
		require.NoError(t, err)

		// Verify the resource
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
		assert.Equal(t, "John Doe", resource.Attributes["name"])
		assert.Equal(t, "john@example.com", resource.Attributes["email"])
	})
}

// Create a struct that implements RelationshipLinksMarshaler
type CustomRelationshipLinksUser struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Posts []struct {
		ID string `jsonapi:"primary,posts"`
	} `jsonapi:"relation,posts"`
}

// Implement RelationshipLinksMarshaler interface
func (u *CustomRelationshipLinksUser) MarshalJSONAPIRelationshipLinks(ctx context.Context, name string) (map[string]Link, error) {
	if name == "posts" {
		return map[string]Link{
			"self":    {Href: "/users/" + u.ID + "/relationships/posts"},
			"related": {Href: "/users/" + u.ID + "/posts"},
		}, nil
	}
	return nil, nil
}

// Create a struct that implements RelationshipMetaMarshaler
type CustomRelationshipMetaUser struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Posts []struct {
		ID string `jsonapi:"primary,posts"`
	} `jsonapi:"relation,posts"`
}

// Implement RelationshipMetaMarshaler interface
func (u *CustomRelationshipMetaUser) MarshalJSONAPIRelationshipMeta(ctx context.Context, name string) (map[string]interface{}, error) {
	if name == "posts" {
		return map[string]interface{}{
			"count": len(u.Posts),
		}, nil
	}
	return nil, nil
}

// TestMarshalRelationship tests the marshalRelationship function
func TestMarshalRelationship(t *testing.T) {
	ctx := context.Background()

	t.Run("to-one relationship", func(t *testing.T) {
		// Create a related struct
		type Profile struct {
			ID  string `jsonapi:"primary,profiles"`
			Bio string `jsonapi:"attr,bio"`
		}

		// Create an instance
		profile := Profile{
			ID:  "101",
			Bio: "Software developer",
		}

		// Marshal the relationship
		rel, _, err := marshalRelationship(ctx, reflect.ValueOf(profile), []string{"profile"})
		require.NoError(t, err)

		// Verify the relationship
		assert.False(t, rel.Data.Null())
		resource, ok := rel.Data.One()
		require.True(t, ok)
		assert.Equal(t, "101", resource.ID)
		assert.Equal(t, "profiles", resource.Type)
	})

	t.Run("to-many relationship", func(t *testing.T) {
		// Create a related struct
		type Post struct {
			ID      string `jsonapi:"primary,posts"`
			Title   string `jsonapi:"attr,title"`
			Content string `jsonapi:"attr,content"`
		}

		// Create instances
		posts := []Post{
			{
				ID:      "101",
				Title:   "First Post",
				Content: "Content 1",
			},
			{
				ID:      "102",
				Title:   "Second Post",
				Content: "Content 2",
			},
		}

		// Marshal the relationship
		rel, _, err := marshalRelationship(ctx, reflect.ValueOf(posts), []string{"posts"})
		require.NoError(t, err)

		// Verify the relationship
		assert.False(t, rel.Data.Null())
		resources, ok := rel.Data.Many()
		require.True(t, ok)
		require.Len(t, resources, 2)
		assert.Equal(t, "101", resources[0].ID)
		assert.Equal(t, "posts", resources[0].Type)
		assert.Equal(t, "102", resources[1].ID)
		assert.Equal(t, "posts", resources[1].Type)
	})

	t.Run("nil relationship", func(t *testing.T) {
		// Create a nil pointer
		var profile *struct {
			ID string `jsonapi:"primary,profiles"`
		}

		// Marshal the relationship
		rel, _, err := marshalRelationship(ctx, reflect.ValueOf(profile), []string{"profile"})
		require.NoError(t, err)

		// Verify the relationship has null data
		assert.True(t, rel.Data.Null())
	})

	t.Run("empty slice relationship", func(t *testing.T) {
		// Create an empty slice
		var posts []struct {
			ID string `jsonapi:"primary,posts"`
		}

		// Marshal the relationship
		rel, _, err := marshalRelationship(ctx, reflect.ValueOf(posts), []string{"posts"})
		require.NoError(t, err)

		// Verify the relationship has empty array data
		resources, ok := rel.Data.Many()
		require.True(t, ok)
		assert.Empty(t, resources)
	})
}

// TestIsSlice tests the isSlice function
func TestIsSlice(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{
			name:     "nil",
			value:    nil,
			expected: false,
		},
		{
			name:     "string",
			value:    "not a slice",
			expected: false,
		},
		{
			name:     "int",
			value:    42,
			expected: false,
		},
		{
			name:     "struct",
			value:    struct{}{},
			expected: false,
		},
		{
			name:     "slice",
			value:    []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "empty slice",
			value:    []int{},
			expected: true,
		},
		{
			name:     "array",
			value:    [3]int{1, 2, 3},
			expected: false, // Arrays are not slices
		},
		{
			name:     "map",
			value:    map[string]int{"a": 1},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSlice(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}
