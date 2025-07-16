package jsonapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	err := unmarshalFromDocument(context.Background(), doc, &user, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

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
	err := unmarshalFromDocument(context.Background(), doc, &user, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result is zero value
	assert.Equal(t, "", user.ID)
	assert.Equal(t, "", user.Name)
	assert.Equal(t, "", user.Email)
}

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
	err := unmarshalFromDocument(context.Background(), doc, &user, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result
	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
}

func TestUnmarshalFromDocument_EmptyMany(t *testing.T) {
	// Test structure
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a document with empty resources
	doc := &Document{
		Data: MultiResource(),
	}

	// Unmarshal into a struct
	var user User
	err := unmarshalFromDocument(context.Background(), doc, &user, &UnmarshalOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected single resource but got none")
}

func TestUnmarshalSliceFromDocument(t *testing.T) {
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

	// Unmarshal into a slice
	var users []User
	err := unmarshalFromDocument(context.Background(), doc, &users, &UnmarshalOptions{})
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

func TestUnmarshalSliceFromDocument_Single(t *testing.T) {
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

	// Unmarshal into a slice
	var users []User
	err := unmarshalFromDocument(context.Background(), doc, &users, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result
	require.Len(t, users, 1)
	assert.Equal(t, "1", users[0].ID)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "john@example.com", users[0].Email)
}

func TestUnmarshalSliceFromDocument_Null(t *testing.T) {
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

	// Unmarshal into a slice
	var users []User
	err := unmarshalFromDocument(context.Background(), doc, &users, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result is nil slice
	assert.Nil(t, users)
}

func TestUnmarshalSliceFromDocument_Empty(t *testing.T) {
	// Test structure
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
	}

	// Create a document with empty resources
	doc := &Document{
		Data: MultiResource(),
	}

	// Unmarshal into a slice
	var users []User
	err := unmarshalFromDocument(context.Background(), doc, &users, &UnmarshalOptions{})
	require.NoError(t, err)

	// Verify the result is empty slice
	assert.Empty(t, users)
}

func TestUnmarshalDocument_Integration(t *testing.T) {
	// Test full unmarshaling from JSON to struct
	jsonData := []byte(`{
		"data": {
			"id": "1",
			"type": "users",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			},
			"relationships": {
				"posts": {
					"data": [
						{
							"id": "101",
							"type": "posts"
						},
						{
							"id": "102",
							"type": "posts"
						}
					]
				},
				"profile": {
					"data": {
						"id": "201",
						"type": "profiles"
					}
				}
			}
		},
		"included": [
			{
				"id": "101",
				"type": "posts",
				"attributes": {
					"title": "First Post",
					"content": "Hello world"
				}
			},
			{
				"id": "102",
				"type": "posts",
				"attributes": {
					"title": "Second Post",
					"content": "More content"
				}
			},
			{
				"id": "201",
				"type": "profiles",
				"attributes": {
					"bio": "Software developer",
					"website": "https://example.com"
				}
			}
		]
	}`)

	// Define the structures
	type Post struct {
		ID      string `jsonapi:"primary,posts"`
		Title   string `jsonapi:"attr,title"`
		Content string `jsonapi:"attr,content"`
	}

	type Profile struct {
		ID      string `jsonapi:"primary,profiles"`
		Bio     string `jsonapi:"attr,bio"`
		Website string `jsonapi:"attr,website"`
	}

	type User struct {
		ID      string   `jsonapi:"primary,users"`
		Name    string   `jsonapi:"attr,name"`
		Email   string   `jsonapi:"attr,email"`
		Posts   []Post   `jsonapi:"relation,posts"`
		Profile Profile  `jsonapi:"relation,profile"`
	}

	// Unmarshal the document
	var user User
	err := Unmarshal(jsonData, &user, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify user data
	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)

	// Verify posts
	require.Len(t, user.Posts, 2)
	assert.Equal(t, "101", user.Posts[0].ID)
	assert.Equal(t, "First Post", user.Posts[0].Title)
	assert.Equal(t, "Hello world", user.Posts[0].Content)
	assert.Equal(t, "102", user.Posts[1].ID)
	assert.Equal(t, "Second Post", user.Posts[1].Title)
	assert.Equal(t, "More content", user.Posts[1].Content)

	// Verify profile
	assert.Equal(t, "201", user.Profile.ID)
	assert.Equal(t, "Software developer", user.Profile.Bio)
	assert.Equal(t, "https://example.com", user.Profile.Website)
}
