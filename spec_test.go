package jsonapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Custom types for testing
type CustomResource struct {
	ID   string
	Name string
}

func (c CustomResource) MarshalJSONAPIResource(ctx context.Context) (Resource, error) {
	return Resource{
		Type: "custom",
		ID:   c.ID,
		Attributes: map[string]interface{}{
			"name":   c.Name,
			"custom": "value",
		},
	}, nil
}

func (c CustomResource) MarshalJSONAPILinks(ctx context.Context) (map[string]Link, error) {
	return map[string]Link{
		"self": {Href: "/custom/" + c.ID},
	}, nil
}

func (c CustomResource) MarshalJSONAPIMeta(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"version": "2.0",
	}, nil
}

// TestSpecificationCompliance tests that the library meets all JSON:API specification requirements
func TestSpecificationCompliance(t *testing.T) {
	t.Run("Document Structure", func(t *testing.T) {
		// Test that Document has all required fields according to spec
		doc := Document{
			Meta:     map[string]interface{}{"version": "1.0"},
			Data:     SingleResource(Resource{ID: "1", Type: "users"}),
			Errors:   []Error{{Status: "400", Code: "BAD_REQUEST", Title: "Bad Request", Detail: "Invalid input"}},
			Links:    map[string]Link{"self": {Href: "/users"}},
			Included: []Resource{{ID: "2", Type: "posts"}},
		}

		jsonData, err := json.Marshal(doc)
		require.NoError(t, err)

		var unmarshaled Document
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.NotNil(t, unmarshaled.Meta)
		assert.NotNil(t, unmarshaled.Links)
		assert.Len(t, unmarshaled.Errors, 1)
		assert.Len(t, unmarshaled.Included, 1)
	})

	t.Run("Resource Structure", func(t *testing.T) {
		// Test that Resource has all required fields according to spec
		resource := Resource{
			ID:            "1",
			Type:          "users",
			Meta:          map[string]interface{}{"version": "1.0"},
			Attributes:    map[string]interface{}{"name": "John"},
			Relationships: map[string]Relationship{"posts": {Data: NullResource()}},
			Links:         map[string]Link{"self": {Href: "/users/1"}},
		}

		// Test Ref() method
		ref := resource.Ref()
		assert.Equal(t, "1", ref.ID)
		assert.Equal(t, "users", ref.Type)
		assert.Nil(t, ref.Attributes)
		assert.Nil(t, ref.Relationships)
		assert.Nil(t, ref.Meta)
		assert.Nil(t, ref.Links)
	})

	t.Run("PrimaryData Variants", func(t *testing.T) {
		// Test single resource
		single := SingleResource(Resource{ID: "1", Type: "users"})
		assert.False(t, single.Null())
		resource, ok := single.One()
		assert.True(t, ok)
		assert.Equal(t, "1", resource.ID)

		// Test multiple resources
		multi := MultiResource(
			Resource{ID: "1", Type: "users"},
			Resource{ID: "2", Type: "users"},
		)
		assert.False(t, multi.Null())
		resources, ok := multi.Many()
		assert.True(t, ok)
		assert.Len(t, resources, 2)

		// Test null resource
		null := NullResource()
		assert.True(t, null.Null())
		_, ok = null.One()
		assert.False(t, ok)
		_, ok = null.Many()
		assert.False(t, ok)

		// Test iterator
		var collected []Resource
		for r := range multi.Iter() {
			collected = append(collected, r)
		}
		assert.Len(t, collected, 2)
	})

	t.Run("Struct Tag Marshaling", func(t *testing.T) {
		type TestStruct struct {
			ID    string `jsonapi:"primary,test-resources"`
			Name  string `jsonapi:"attr,name"`
			Email string `jsonapi:"attr,email,omitempty"`
		}

		// Test with all fields
		obj := TestStruct{ID: "1", Name: "Test", Email: "test@example.com"}
		data, err := Marshal(obj)
		require.NoError(t, err)

		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "test-resources", resource.Type)
		assert.Equal(t, "Test", resource.Attributes["name"])
		assert.Equal(t, "test@example.com", resource.Attributes["email"])

		// Test omitempty
		objEmpty := TestStruct{ID: "2", Name: "Test2"} // Email is empty
		data, err = Marshal(objEmpty)
		require.NoError(t, err)

		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		resource, ok = doc.Data.One()
		assert.True(t, ok)
		assert.NotContains(t, resource.Attributes, "email")
	})

	t.Run("Relationship Marshaling", func(t *testing.T) {
		type Post struct {
			ID    string `jsonapi:"primary,posts"`
			Title string `jsonapi:"attr,title"`
		}

		type User struct {
			ID    string `jsonapi:"primary,users"`
			Name  string `jsonapi:"attr,name"`
			Posts []Post `jsonapi:"relation,posts"`
		}

		user := User{
			ID:   "1",
			Name: "John",
			Posts: []Post{
				{ID: "1", Title: "Post 1"},
				{ID: "2", Title: "Post 2"},
			},
		}

		// Test without included resources
		data, err := Marshal(user)
		require.NoError(t, err)

		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Contains(t, resource.Relationships, "posts")

		postsRel := resource.Relationships["posts"]
		postRefs, ok := postsRel.Data.Many()
		assert.True(t, ok)
		assert.Len(t, postRefs, 2)
		assert.Equal(t, "1", postRefs[0].ID)
		assert.Equal(t, "posts", postRefs[0].Type)

		// Test with included resources
		data, err = Marshal(user, IncludeRelatedResources())
		require.NoError(t, err)

		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		assert.Len(t, doc.Included, 2)
		assert.Equal(t, "Post 1", doc.Included[0].Attributes["title"])
	})

	t.Run("Embedded Struct Support", func(t *testing.T) {
		type Timestamps struct {
			CreatedAt time.Time `jsonapi:"attr,created_at"`
			UpdatedAt time.Time `jsonapi:"attr,updated_at"`
		}

		type User struct {
			Timestamps
			ID   string `jsonapi:"primary,users"`
			Name string `jsonapi:"attr,name"`
		}

		now := time.Now()
		user := User{
			ID:   "1",
			Name: "John",
			Timestamps: Timestamps{
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
		assert.Contains(t, resource.Attributes, "created_at")
		assert.Contains(t, resource.Attributes, "updated_at")
		assert.Contains(t, resource.Attributes, "name")
	})

	t.Run("Custom Marshaling Interfaces", func(t *testing.T) {
		obj := CustomResource{ID: "1", Name: "Custom"}
		data, err := Marshal(obj)
		require.NoError(t, err)

		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Equal(t, "custom", resource.Type)
		assert.Equal(t, "value", resource.Attributes["custom"])
		assert.Equal(t, "/custom/1", resource.Links["self"].Href)
		assert.Equal(t, "2.0", resource.Meta["version"])
	})

	t.Run("Context Support", func(t *testing.T) {
		type User struct {
			ID   string `jsonapi:"primary,users"`
			Name string `jsonapi:"attr,name"`
		}

		ctx := context.WithValue(context.Background(), "test", "value")
		user := User{ID: "1", Name: "John"}

		data, err := MarshalWithContext(ctx, user)
		require.NoError(t, err)

		var doc Document
		err = json.Unmarshal(data, &doc)
		require.NoError(t, err)

		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Equal(t, "1", resource.ID)
	})

	t.Run("Marshaling Options", func(t *testing.T) {
		type User struct {
			ID   string `jsonapi:"primary,users"`
			Name string `jsonapi:"attr,name"`
		}

		user := User{ID: "1", Name: "John"}

		// Test custom marshaler
		data, err := Marshal(user, WithMarshaler(func(out interface{}) ([]byte, error) {
			return json.MarshalIndent(out, "", "  ")
		}))
		require.NoError(t, err)
		assert.Contains(t, string(data), "\n") // Should be indented
	})

	t.Run("Error Handling", func(t *testing.T) {
		// Test nil input
		_, err := Marshal(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot marshal nil value")

		// Test nil pointer
		var user *struct{}
		_, err = Marshal(user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot marshal nil value")

		// Test non-struct
		_, err = Marshal("not a struct")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected struct")
	})

	t.Run("Thread Safety", func(t *testing.T) {
		type User struct {
			ID   string `jsonapi:"primary,users"`
			Name string `jsonapi:"attr,name"`
		}

		user := User{ID: "1", Name: "John"}

		// Run multiple goroutines to test thread safety
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_, err := Marshal(user)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestUnmarshalingSpecificationCompliance tests that the library meets all JSON:API specification requirements for unmarshaling
func TestUnmarshalingSpecificationCompliance(t *testing.T) {
	t.Run("Core Unmarshaling Functions", func(t *testing.T) {
		// Test Unmarshal function
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)
		assert.Equal(t, "1", user.ID)
		assert.Equal(t, "John Doe", user.Name)

		// Test UnmarshalWithContext function
		ctx := context.WithValue(context.Background(), "test", "value")
		var user2 UnmarshalUser
		err = UnmarshalWithContext(ctx, []byte(jsonData), &user2)
		require.NoError(t, err)
		assert.Equal(t, "1", user2.ID)

		// Test UnmarshalDocument function
		doc, err := UnmarshalDocument([]byte(jsonData))
		require.NoError(t, err)
		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
	})

	t.Run("Unmarshaling Options", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				},
				"relationships": {
					"posts": {
						"data": [{"type": "posts", "id": "1"}]
					}
				}
			},
			"included": [
				{
					"type": "posts",
					"id": "1",
					"attributes": {
						"title": "Test Post"
					}
				}
			]
		}`

		// Test WithUnmarshaler option
		var user1 UnmarshalUserWithPosts
		err := Unmarshal([]byte(jsonData), &user1, WithUnmarshaler(func(data []byte, out interface{}) error {
			return json.Unmarshal(data, out)
		}))
		require.NoError(t, err)
		assert.Equal(t, "John Doe", user1.Name)

		// Test PopulateFromIncluded option
		var user2 UnmarshalUserWithPosts
		err = Unmarshal([]byte(jsonData), &user2, PopulateFromIncluded())
		require.NoError(t, err)
		assert.Len(t, user2.Posts, 1)
		assert.Equal(t, "Test Post", user2.Posts[0].Title)

		// Test StrictMode option
		invalidTypeData := `{
			"data": {
				"type": "posts",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`
		var user3 UnmarshalUser
		err = Unmarshal([]byte(invalidTypeData), &user3, StrictMode())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource type mismatch")
	})

	t.Run("Custom Unmarshaling Interfaces", func(t *testing.T) {
		// Test ResourceUnmarshaler
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"custom_field": "custom_value"
				}
			}
		}`

		var user CustomUnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)
		assert.Equal(t, "1", user.ID)
		assert.Equal(t, "John Doe", user.Name)
		assert.Equal(t, "custom_value", user.CustomField)

		// Test LinksUnmarshaler
		linksData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				},
				"links": {
					"self": {"href": "/users/1"}
				}
			}
		}`

		var userWithLinks UnmarshalUserWithLinks
		err = Unmarshal([]byte(linksData), &userWithLinks)
		require.NoError(t, err)
		assert.Equal(t, "/users/1", userWithLinks.Links["self"])

		// Test MetaUnmarshaler
		metaData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				},
				"meta": {
					"version": "1.0"
				}
			}
		}`

		var userWithMeta UnmarshalUserWithMeta
		err = Unmarshal([]byte(metaData), &userWithMeta)
		require.NoError(t, err)
		assert.Equal(t, "1.0", userWithMeta.MetaMap["version"])
	})

	t.Run("Struct Tag Unmarshaling", func(t *testing.T) {
		// Test primary key unmarshaling
		jsonData := `{
			"data": {
				"type": "users",
				"id": "123",
				"attributes": {
					"name": "John Doe",
					"email": "john@example.com",
					"age": 30
				}
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		// Primary key
		assert.Equal(t, "123", user.ID)

		// Attributes
		assert.Equal(t, "John Doe", user.Name)
		assert.Equal(t, "john@example.com", user.Email)
		assert.Equal(t, 30, user.Age)

		// Test relationships
		relationshipData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				},
				"relationships": {
					"posts": {
						"data": [
							{"type": "posts", "id": "1"},
							{"type": "posts", "id": "2"}
						]
					}
				}
			}
		}`

		var userWithPosts UnmarshalUserWithPosts
		err = Unmarshal([]byte(relationshipData), &userWithPosts)
		require.NoError(t, err)
		assert.Len(t, userWithPosts.Posts, 2)
		assert.Equal(t, "1", userWithPosts.Posts[0].ID)
		assert.Equal(t, "2", userWithPosts.Posts[1].ID)
	})

	t.Run("Type Conversion Support", func(t *testing.T) {
		// Test various type conversions
		jsonData := `{
			"data": {
				"type": "test",
				"id": "1",
				"attributes": {
					"string_field": "test",
					"int_from_string": "42",
					"int_from_float": 42.7,
					"float_from_int": 42,
					"bool_from_string": "true"
				}
			}
		}`

		type TestStruct struct {
			ID             string  `jsonapi:"primary,test"`
			StringField    string  `jsonapi:"attr,string_field"`
			IntFromString  int     `jsonapi:"attr,int_from_string"`
			IntFromFloat   int     `jsonapi:"attr,int_from_float"`
			FloatFromInt   float64 `jsonapi:"attr,float_from_int"`
			BoolFromString bool    `jsonapi:"attr,bool_from_string"`
		}

		var test TestStruct
		err := Unmarshal([]byte(jsonData), &test)
		require.NoError(t, err)

		assert.Equal(t, "test", test.StringField)
		assert.Equal(t, 42, test.IntFromString)
		assert.Equal(t, 42, test.IntFromFloat)
		assert.Equal(t, 42.0, test.FloatFromInt)
		assert.Equal(t, true, test.BoolFromString)
	})

	t.Run("Embedded Struct Support", func(t *testing.T) {
		now := time.Now()
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"created_at": "` + now.Format(time.RFC3339) + `",
					"updated_at": "` + now.Format(time.RFC3339) + `"
				}
			}
		}`

		var user UnmarshalUserWithTimestamp
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		assert.Equal(t, "1", user.ID)
		assert.Equal(t, "John Doe", user.Name)
		assert.WithinDuration(t, now, user.CreatedAt, time.Second)
		assert.WithinDuration(t, now, user.UpdatedAt, time.Second)
	})

	t.Run("Document Structure Support", func(t *testing.T) {
		// Test single resource document
		singleData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			},
			"meta": {
				"version": "1.0"
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(singleData), &user)
		require.NoError(t, err)
		assert.Equal(t, "1", user.ID)

		// Test multiple resource document
		multipleData := `{
			"data": [
				{
					"type": "users",
					"id": "1",
					"attributes": {
						"name": "John Doe"
					}
				},
				{
					"type": "users",
					"id": "2",
					"attributes": {
						"name": "Jane Doe"
					}
				}
			]
		}`

		var users []UnmarshalUser
		err = Unmarshal([]byte(multipleData), &users)
		require.NoError(t, err)
		assert.Len(t, users, 2)

		// Test null data document
		nullData := `{
			"data": null
		}`

		var nullUser UnmarshalUser
		err = Unmarshal([]byte(nullData), &nullUser)
		require.NoError(t, err)
		assert.Equal(t, "", nullUser.ID) // Should be zero value
	})

	t.Run("Relationship Population", func(t *testing.T) {
		// Test relationship population from included resources
		compoundData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				},
				"relationships": {
					"posts": {
						"data": [
							{"type": "posts", "id": "1"},
							{"type": "posts", "id": "2"}
						]
					}
				}
			},
			"included": [
				{
					"type": "posts",
					"id": "1",
					"attributes": {
						"title": "First Post",
						"body": "Content 1"
					}
				},
				{
					"type": "posts",
					"id": "2",
					"attributes": {
						"title": "Second Post",
						"body": "Content 2"
					}
				}
			]
		}`

		var user UnmarshalUserWithPosts
		err := Unmarshal([]byte(compoundData), &user, PopulateFromIncluded())
		require.NoError(t, err)

		assert.Equal(t, "1", user.ID)
		assert.Equal(t, "John Doe", user.Name)
		assert.Len(t, user.Posts, 2)
		assert.Equal(t, "1", user.Posts[0].ID)
		assert.Equal(t, "First Post", user.Posts[0].Title)
		assert.Equal(t, "Content 1", user.Posts[0].Body)
		assert.Equal(t, "2", user.Posts[1].ID)
		assert.Equal(t, "Second Post", user.Posts[1].Title)
		assert.Equal(t, "Content 2", user.Posts[1].Body)
	})

	t.Run("Error Handling", func(t *testing.T) {
		// Test various error conditions

		// Invalid JSON
		_, err := UnmarshalDocument([]byte(`{invalid json}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON:API document")

		// Nil output
		err = Unmarshal([]byte(`{"data": null}`), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal into nil value")

		// Non-pointer output
		var user UnmarshalUser
		err = Unmarshal([]byte(`{"data": null}`), user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out parameter must be a pointer")

		// Nil pointer
		var userPtr *UnmarshalUser
		err = Unmarshal([]byte(`{"data": null}`), userPtr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal into nil pointer")

		// Type mismatch in strict mode
		mismatchData := `{
			"data": {
				"type": "posts",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		var user2 UnmarshalUser
		err = Unmarshal([]byte(mismatchData), &user2, StrictMode())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource type mismatch")
	})

	t.Run("Thread Safety", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		// Run multiple goroutines to test thread safety
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				var user UnmarshalUser
				err := Unmarshal([]byte(jsonData), &user)
				assert.NoError(t, err)
				assert.Equal(t, "1", user.ID)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("Context Support", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		ctx := context.WithValue(context.Background(), "test", "value")
		var user UnmarshalUser
		err := UnmarshalWithContext(ctx, []byte(jsonData), &user)
		require.NoError(t, err)
		assert.Equal(t, "1", user.ID)
	})

	t.Run("Pointer and Slice Handling", func(t *testing.T) {
		// Test pointer slices
		jsonData := `{
			"data": [
				{
					"type": "users",
					"id": "1",
					"attributes": {
						"name": "John Doe"
					}
				},
				{
					"type": "users",
					"id": "2",
					"attributes": {
						"name": "Jane Doe"
					}
				}
			]
		}`

		var users []*UnmarshalUser
		err := Unmarshal([]byte(jsonData), &users)
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "1", users[0].ID)
		assert.Equal(t, "2", users[1].ID)

		// Test single resource to slice conversion
		singleData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		var userSlice []UnmarshalUser
		err = Unmarshal([]byte(singleData), &userSlice)
		require.NoError(t, err)
		assert.Len(t, userSlice, 1)
		assert.Equal(t, "1", userSlice[0].ID)

		// Test empty array
		emptyData := `{
			"data": []
		}`

		var emptyUsers []UnmarshalUser
		err = Unmarshal([]byte(emptyData), &emptyUsers)
		require.NoError(t, err)
		assert.Len(t, emptyUsers, 0)
		assert.NotNil(t, emptyUsers) // Should be empty slice, not nil
	})
}
