package jsonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for unmarshaling
type UnmarshalUser struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Email string `jsonapi:"attr,email"`
	Age   int    `jsonapi:"attr,age"`
}

type UnmarshalPost struct {
	ID    string `jsonapi:"primary,posts"`
	Title string `jsonapi:"attr,title"`
	Body  string `jsonapi:"attr,body"`
}

type UnmarshalUserWithPosts struct {
	ID    string          `jsonapi:"primary,users"`
	Name  string          `jsonapi:"attr,name"`
	Posts []UnmarshalPost `jsonapi:"relation,posts"`
}

type UnmarshalUserWithProfile struct {
	ID      string            `jsonapi:"primary,users"`
	Name    string            `jsonapi:"attr,name"`
	Profile *UnmarshalProfile `jsonapi:"relation,profile"`
}

type UnmarshalProfile struct {
	ID  string `jsonapi:"primary,profiles"`
	Bio string `jsonapi:"attr,bio"`
}

type UnmarshalAddress struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type UnmarshalUserWithAddress struct {
	ID      string           `jsonapi:"primary,users"`
	Name    string           `jsonapi:"attr,name"`
	Address UnmarshalAddress `jsonapi:"attr,address"`
}

type UnmarshalTimestamp struct {
	CreatedAt time.Time `jsonapi:"attr,created_at"`
	UpdatedAt time.Time `jsonapi:"attr,updated_at"`
}

type UnmarshalUserWithTimestamp struct {
	UnmarshalTimestamp
	ID   string `jsonapi:"primary,users"`
	Name string `jsonapi:"attr,name"`
}

// Custom unmarshaler test struct
type CustomUnmarshalUser struct {
	ID          string
	Name        string
	CustomField string
}

func (u *CustomUnmarshalUser) UnmarshalJSONAPIResource(ctx context.Context, resource Resource) error {
	u.ID = resource.ID
	if name, ok := resource.Attributes["name"].(string); ok {
		u.Name = name
	}
	if customField, ok := resource.Attributes["custom_field"].(string); ok {
		u.CustomField = customField
	}
	return nil
}

// Links unmarshaler test struct
type UnmarshalUserWithLinks struct {
	ID    string            `jsonapi:"primary,users"`
	Name  string            `jsonapi:"attr,name"`
	Links map[string]string // Store links as simple map
}

func (u *UnmarshalUserWithLinks) UnmarshalJSONAPILinks(ctx context.Context, links map[string]Link) error {
	u.Links = make(map[string]string)
	for key, link := range links {
		u.Links[key] = link.Href
	}
	return nil
}

// Meta unmarshaler test struct
type UnmarshalUserWithMeta struct {
	ID      string                 `jsonapi:"primary,users"`
	Name    string                 `jsonapi:"attr,name"`
	MetaMap map[string]interface{} // Store meta
}

func (u *UnmarshalUserWithMeta) UnmarshalJSONAPIMeta(ctx context.Context, meta map[string]interface{}) error {
	u.MetaMap = meta
	return nil
}

func TestUnmarshal_SingleResource(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
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

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

func TestUnmarshal_MultipleResources(t *testing.T) {
	jsonData := `{
		"data": [
			{
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"email": "john@example.com",
					"age": 30
				}
			},
			{
				"type": "users",
				"id": "2",
				"attributes": {
					"name": "Jane Doe",
					"email": "jane@example.com",
					"age": 25
				}
			}
		]
	}`

	var users []UnmarshalUser
	err := Unmarshal([]byte(jsonData), &users)
	require.NoError(t, err)

	assert.Len(t, users, 2)
	assert.Equal(t, "1", users[0].ID)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "2", users[1].ID)
	assert.Equal(t, "Jane Doe", users[1].Name)
}

func TestUnmarshal_NullData(t *testing.T) {
	jsonData := `{
		"data": null
	}`

	var user UnmarshalUser
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	// Should be zero value
	assert.Equal(t, "", user.ID)
	assert.Equal(t, "", user.Name)
	assert.Equal(t, "", user.Email)
	assert.Equal(t, 0, user.Age)
}

func TestUnmarshal_NullDataSlice(t *testing.T) {
	jsonData := `{
		"data": null
	}`

	var users []UnmarshalUser
	err := Unmarshal([]byte(jsonData), &users)
	require.NoError(t, err)

	assert.Nil(t, users)
}

func TestUnmarshal_NestedStructAttributes(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe",
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}
		}
	}`

	var user UnmarshalUserWithAddress
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "123 Main St", user.Address.Street)
	assert.Equal(t, "Anytown", user.Address.City)
}

func TestUnmarshal_EmbeddedStructs(t *testing.T) {
	now := time.Now()
	jsonData := fmt.Sprintf(`{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe",
				"created_at": "%s",
				"updated_at": "%s"
			}
		}
	}`, now.Format(time.RFC3339), now.Format(time.RFC3339))

	var user UnmarshalUserWithTimestamp
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	// Time parsing might have slight differences, so check they're close
	assert.WithinDuration(t, now, user.CreatedAt, time.Second)
	assert.WithinDuration(t, now, user.UpdatedAt, time.Second)
}

func TestUnmarshal_Relationships(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"relationships": {
				"posts": {
					"data": [
						{
							"type": "posts",
							"id": "1"
						},
						{
							"type": "posts",
							"id": "2"
						}
					]
				}
			}
		}
	}`

	var user UnmarshalUserWithPosts
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Len(t, user.Posts, 2)
	assert.Equal(t, "1", user.Posts[0].ID)
	assert.Equal(t, "2", user.Posts[1].ID)
	// Attributes should be empty since not populated from included
	assert.Equal(t, "", user.Posts[0].Title)
	assert.Equal(t, "", user.Posts[1].Title)
}

func TestUnmarshal_RelationshipsWithIncluded(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"relationships": {
				"posts": {
					"data": [
						{
							"type": "posts",
							"id": "1"
						},
						{
							"type": "posts",
							"id": "2"
						}
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
	err := Unmarshal([]byte(jsonData), &user, PopulateFromIncluded())
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
}

func TestUnmarshal_SingleRelationship(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"relationships": {
				"profile": {
					"data": {
						"type": "profiles",
						"id": "1"
					}
				}
			}
		},
		"included": [
			{
				"type": "profiles",
				"id": "1",
				"attributes": {
					"bio": "Software Developer"
				}
			}
		]
	}`

	var user UnmarshalUserWithProfile
	err := Unmarshal([]byte(jsonData), &user, PopulateFromIncluded())
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	require.NotNil(t, user.Profile)
	assert.Equal(t, "1", user.Profile.ID)
	assert.Equal(t, "Software Developer", user.Profile.Bio)
}

func TestUnmarshal_NullRelationship(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"relationships": {
				"profile": {
					"data": null
				}
			}
		}
	}`

	var user UnmarshalUserWithProfile
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Nil(t, user.Profile)
}

func TestUnmarshal_CustomResourceUnmarshaler(t *testing.T) {
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
}

func TestUnmarshal_CustomLinksUnmarshaler(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"links": {
				"self": {
					"href": "/users/1"
				}
			}
		}
	}`

	var user UnmarshalUserWithLinks
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "/users/1", user.Links["self"])
}

func TestUnmarshal_CustomMetaUnmarshaler(t *testing.T) {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe"
			},
			"meta": {
				"version": "1.0",
				"count": 42
			}
		}
	}`

	var user UnmarshalUserWithMeta
	err := Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "1.0", user.MetaMap["version"])
	assert.Equal(t, float64(42), user.MetaMap["count"]) // JSON numbers are float64
}

func TestUnmarshal_WithCustomUnmarshaler(t *testing.T) {
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
	err := Unmarshal([]byte(jsonData), &user, WithUnmarshaler(func(data []byte, out interface{}) error {
		return json.Unmarshal(data, out)
	}))
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
}

func TestUnmarshal_StrictMode(t *testing.T) {
	t.Run("valid document", func(t *testing.T) {
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
		err := Unmarshal([]byte(jsonData), &user, StrictMode())
		require.NoError(t, err)

		assert.Equal(t, "1", user.ID)
		assert.Equal(t, "John Doe", user.Name)
	})

	t.Run("type mismatch", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "posts",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user, StrictMode())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource type mismatch")
	})
}

func TestUnmarshal_TypeConversion(t *testing.T) {
	t.Run("string to int", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"age": "30"
				}
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		assert.Equal(t, 30, user.Age)
	})

	t.Run("float to int", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"age": 30.5
				}
			}
		}`

		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		assert.Equal(t, 30, user.Age)
	})
}

func TestUnmarshalWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "test", "value")

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
	err := UnmarshalWithContext(ctx, []byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "1", user.ID)
	assert.Equal(t, "John Doe", user.Name)
}

func TestUnmarshalDocument(t *testing.T) {
	jsonData := `{
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

	doc, err := UnmarshalDocument([]byte(jsonData))
	require.NoError(t, err)

	assert.NotNil(t, doc.Meta)
	assert.Equal(t, "1.0", doc.Meta["version"])

	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
}

func TestUnmarshal_ErrorCases(t *testing.T) {
	t.Run("nil output", func(t *testing.T) {
		jsonData := `{"data": null}`
		err := Unmarshal([]byte(jsonData), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal into nil value")
	})

	t.Run("non-pointer output", func(t *testing.T) {
		jsonData := `{"data": null}`
		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out parameter must be a pointer")
	})

	t.Run("nil pointer", func(t *testing.T) {
		jsonData := `{"data": null}`
		var user *UnmarshalUser
		err := Unmarshal([]byte(jsonData), user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal into nil pointer")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonData := `{invalid json}`
		var user UnmarshalUser
		err := Unmarshal([]byte(jsonData), &user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON:API document")
	})

	t.Run("single resource to slice mismatch", func(t *testing.T) {
		jsonData := `{
			"data": {
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe"
				}
			}
		}`

		var users []UnmarshalUser
		err := Unmarshal([]byte(jsonData), &users)
		require.NoError(t, err)

		// Should work - single resource becomes slice with one element
		assert.Len(t, users, 1)
		assert.Equal(t, "1", users[0].ID)
	})
}

func TestUnmarshal_PointerSlices(t *testing.T) {
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
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "2", users[1].ID)
	assert.Equal(t, "Jane Doe", users[1].Name)
}

func TestUnmarshal_EmptyArray(t *testing.T) {
	jsonData := `{
		"data": []
	}`

	var users []UnmarshalUser
	err := Unmarshal([]byte(jsonData), &users)
	require.NoError(t, err)

	assert.Len(t, users, 0)
	assert.NotNil(t, users) // Should be empty slice, not nil
}
