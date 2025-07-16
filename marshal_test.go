package jsonapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs
type User struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Email string `jsonapi:"attr,email,omitempty"`
}

type Post struct {
	ID    string `jsonapi:"primary,posts"`
	Title string `jsonapi:"attr,title"`
	Body  string `jsonapi:"attr,body,omitempty"`
}

type UserWithPosts struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Posts []Post `jsonapi:"relation,posts"`
}

type UserWithProfile struct {
	ID      string   `jsonapi:"primary,users"`
	Name    string   `jsonapi:"attr,name"`
	Profile *Profile `jsonapi:"relation,profile,omitempty"`
}

type Profile struct {
	ID  string `jsonapi:"primary,profiles"`
	Bio string `jsonapi:"attr,bio"`
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type UserWithAddress struct {
	ID      string  `jsonapi:"primary,users"`
	Name    string  `jsonapi:"attr,name"`
	Address Address `jsonapi:"attr,address"`
}

type Timestamp struct {
	CreatedAt time.Time `jsonapi:"attr,created_at"`
	UpdatedAt time.Time `jsonapi:"attr,updated_at"`
}

type UserWithTimestamp struct {
	Timestamp
	ID   string `jsonapi:"primary,users"`
	Name string `jsonapi:"attr,name"`
}

// Custom marshaler test struct
type CustomUser struct {
	ID   string
	Name string
}

func (u CustomUser) MarshalJSONAPIResource(ctx context.Context) (Resource, error) {
	return Resource{
		Type: "users",
		ID:   u.ID,
		Attributes: map[string]interface{}{
			"name":         u.Name,
			"custom_field": "custom_value",
		},
	}, nil
}

// Links marshaler test struct
type UserWithLinks struct {
	ID   string `jsonapi:"primary,users"`
	Name string `jsonapi:"attr,name"`
}

func (u UserWithLinks) MarshalJSONAPILinks(ctx context.Context) (map[string]Link, error) {
	return map[string]Link{
		"self": {Href: "/users/" + u.ID},
	}, nil
}

// Meta marshaler test struct
type UserWithMeta struct {
	ID   string `jsonapi:"primary,users"`
	Name string `jsonapi:"attr,name"`
}

func (u UserWithMeta) MarshalJSONAPIMeta(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"version": "1.0",
	}, nil
}

func TestMarshal_SingleResource(t *testing.T) {
	user := User{
		ID:    "1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
}

func TestMarshal_MultipleResources(t *testing.T) {
	users := []User{
		{ID: "1", Name: "John Doe", Email: "john@example.com"},
		{ID: "2", Name: "Jane Doe", Email: "jane@example.com"},
	}

	data, err := Marshal(users)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resources, ok := doc.Data.Many()
	assert.True(t, ok)
	assert.Len(t, resources, 2)

	assert.Equal(t, "1", resources[0].ID)
	assert.Equal(t, "users", resources[0].Type)
	assert.Equal(t, "John Doe", resources[0].Attributes["name"])

	assert.Equal(t, "2", resources[1].ID)
	assert.Equal(t, "users", resources[1].Type)
	assert.Equal(t, "Jane Doe", resources[1].Attributes["name"])
}

func TestMarshal_OmitEmpty(t *testing.T) {
	user := User{
		ID:   "1",
		Name: "John Doe",
		// Email is empty and should be omitted
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.NotContains(t, resource.Attributes, "email")
}

func TestMarshal_NestedStructAttributes(t *testing.T) {
	user := UserWithAddress{
		ID:   "1",
		Name: "John Doe",
		Address: Address{
			Street: "123 Main St",
			City:   "Anytown",
		},
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "John Doe", resource.Attributes["name"])

	address, ok := resource.Attributes["address"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "123 Main St", address["street"])
	assert.Equal(t, "Anytown", address["city"])
}

func TestMarshal_EmbeddedStructs(t *testing.T) {
	now := time.Now()
	user := UserWithTimestamp{
		ID:   "1",
		Name: "John Doe",
		Timestamp: Timestamp{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Contains(t, resource.Attributes, "created_at")
	assert.Contains(t, resource.Attributes, "updated_at")
}

func TestMarshal_Relationships(t *testing.T) {
	user := UserWithPosts{
		ID:   "1",
		Name: "John Doe",
		Posts: []Post{
			{ID: "1", Title: "First Post", Body: "Content 1"},
			{ID: "2", Title: "Second Post", Body: "Content 2"},
		},
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "John Doe", resource.Attributes["name"])

	postsRel, exists := resource.Relationships["posts"]
	assert.True(t, exists)

	postRefs, ok := postsRel.Data.Many()
	assert.True(t, ok)
	assert.Len(t, postRefs, 2)
	assert.Equal(t, "1", postRefs[0].ID)
	assert.Equal(t, "posts", postRefs[0].Type)
	assert.Equal(t, "2", postRefs[1].ID)
	assert.Equal(t, "posts", postRefs[1].Type)
}

func TestMarshal_RelationshipsWithIncluded(t *testing.T) {
	user := UserWithPosts{
		ID:   "1",
		Name: "John Doe",
		Posts: []Post{
			{ID: "1", Title: "First Post", Body: "Content 1"},
			{ID: "2", Title: "Second Post", Body: "Content 2"},
		},
	}

	data, err := Marshal(user, IncludeRelatedResources())
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Check included resources
	assert.Len(t, doc.Included, 2)
	assert.Equal(t, "1", doc.Included[0].ID)
	assert.Equal(t, "posts", doc.Included[0].Type)
	assert.Equal(t, "First Post", doc.Included[0].Attributes["title"])
	assert.Equal(t, "Content 1", doc.Included[0].Attributes["body"])
}

func TestMarshal_NilPointerRelationship(t *testing.T) {
	user := UserWithProfile{
		ID:      "1",
		Name:    "John Doe",
		Profile: nil, // nil pointer should create null relationship
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)

	// With omitempty, nil relationships should be omitted
	assert.NotContains(t, resource.Relationships, "profile")
}

func TestMarshal_NonNilPointerRelationship(t *testing.T) {
	user := UserWithProfile{
		ID:   "1",
		Name: "John Doe",
		Profile: &Profile{
			ID:  "1",
			Bio: "Software Developer",
		},
	}

	data, err := Marshal(user, IncludeRelatedResources())
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)

	profileRel, exists := resource.Relationships["profile"]
	assert.True(t, exists)

	profileRef, ok := profileRel.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", profileRef.ID)
	assert.Equal(t, "profiles", profileRef.Type)

	// Check included
	assert.Len(t, doc.Included, 1)
	assert.Equal(t, "Software Developer", doc.Included[0].Attributes["bio"])
}

func TestMarshal_CustomResourceMarshaler(t *testing.T) {
	user := CustomUser{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "custom_value", resource.Attributes["custom_field"])
}

func TestMarshal_CustomLinksMarshaler(t *testing.T) {
	user := UserWithLinks{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Contains(t, resource.Links, "self")
	assert.Equal(t, "/users/1", resource.Links["self"].Href)
}

func TestMarshal_CustomMetaMarshaler(t *testing.T) {
	user := UserWithMeta{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Contains(t, resource.Meta, "version")
	assert.Equal(t, "1.0", resource.Meta["version"])
}

func TestMarshal_WithCustomMarshaler(t *testing.T) {
	user := User{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := Marshal(user, WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	require.NoError(t, err)

	// Should be pretty-printed JSON
	assert.Contains(t, string(data), "\n")
	assert.Contains(t, string(data), "  ")
}

func TestMarshal_ErrorCases(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		_, err := Marshal(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot marshal nil value")
	})

	t.Run("nil pointer", func(t *testing.T) {
		var user *User
		_, err := Marshal(user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot marshal nil value")
	})

	t.Run("non-struct type", func(t *testing.T) {
		_, err := Marshal("not a struct")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected struct")
	})
}

func TestMarshalWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "test", "value")

	user := User{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := MarshalWithContext(ctx, user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
}

func TestMarshal_PointerToStruct(t *testing.T) {
	user := &User{
		ID:   "1",
		Name: "John Doe",
	}

	data, err := Marshal(user)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
}

func TestMarshal_EmptySlice(t *testing.T) {
	var users []User

	data, err := Marshal(users)
	require.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	resources, ok := doc.Data.Many()
	assert.True(t, ok)
	assert.Len(t, resources, 0)
}

func TestMarshalDocument(t *testing.T) {
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

	// Test that MarshalDocument is exported and works correctly
	doc, err := MarshalDocument(context.Background(), user)
	require.NoError(t, err, "MarshalDocument should not return an error")
	require.NotNil(t, doc, "MarshalDocument should return a non-nil document")

	// Check that the document has the correct data
	resource, ok := doc.Data.One()
	require.True(t, ok, "Expected document to have a single resource")
	assert.Equal(t, "users", resource.Type, "Resource type should be 'users'")
	assert.Equal(t, "1", resource.ID, "Resource ID should be '1'")

	// Check attributes
	name, ok := resource.Attributes["name"].(string)
	require.True(t, ok, "Expected name attribute to be a string")
	assert.Equal(t, "John Doe", name, "Name attribute should be 'John Doe'")

	email, ok := resource.Attributes["email"].(string)
	require.True(t, ok, "Expected email attribute to be a string")
	assert.Equal(t, "john@example.com", email, "Email attribute should be 'john@example.com'")

	// Test with a collection
	users := []User{
		{ID: "1", Name: "John Doe", Email: "john@example.com"},
		{ID: "2", Name: "Jane Doe", Email: "jane@example.com"},
	}

	doc, err = MarshalDocument(context.Background(), users)
	require.NoError(t, err, "MarshalDocument should not return an error for collection")
	require.NotNil(t, doc, "MarshalDocument should return a non-nil document for collection")

	// Verify the document structure for collection
	resources, ok := doc.Data.Many()
	require.True(t, ok, "Expected document to have multiple resources")
	assert.Len(t, resources, 2, "Expected 2 resources in the collection")

	// Test with custom marshaler option
	doc, err = MarshalDocument(
		context.Background(),
		user,
		WithMarshaler(func(out interface{}) ([]byte, error) {
			return json.MarshalIndent(out, "", "  ")
		}),
	)
	require.NoError(t, err, "MarshalDocument should not return an error with custom marshaler")
	require.NotNil(t, doc, "MarshalDocument should return a non-nil document with custom marshaler")
}
