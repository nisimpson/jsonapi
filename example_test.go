package jsonapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nisimpson/jsonapi"
)

// Example structs
type User struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Email string `jsonapi:"attr,email,omitempty"`
}

type Post struct {
	ID    string `jsonapi:"primary,posts"`
	Title string `jsonapi:"attr,title"`
	Body  string `jsonapi:"attr,body"`
}

type UserWithPosts struct {
	ID    string `jsonapi:"primary,users"`
	Name  string `jsonapi:"attr,name"`
	Posts []Post `jsonapi:"relation,posts"`
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

// Custom marshaler example
type CustomUser struct {
	ID   string
	Name string
}

func (u CustomUser) MarshalJSONAPIResource(ctx context.Context) (jsonapi.Resource, error) {
	return jsonapi.Resource{
		Type: "users",
		ID:   u.ID,
		Attributes: map[string]interface{}{
			"name":         u.Name,
			"custom_field": "custom_value",
		},
	}, nil
}

// Custom unmarshaler example
type CustomUnmarshalUser struct {
	ID          string
	Name        string
	CustomField string
}

func (u *CustomUnmarshalUser) UnmarshalJSONAPIResource(ctx context.Context, resource jsonapi.Resource) error {
	u.ID = resource.ID
	if name, ok := resource.Attributes["name"].(string); ok {
		u.Name = name
	}
	if customField, ok := resource.Attributes["custom_field"].(string); ok {
		u.CustomField = customField
	}
	return nil
}

// ExampleMarshalSingleResource demonstrates marshaling a single resource to JSON:API format
func ExampleMarshal_singleResource() {
	user := User{
		ID:    "1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	data, _ := jsonapi.Marshal(user, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))
	// Output:
	// {
	//   "data": {
	//     "id": "1",
	//     "type": "users",
	//     "attributes": {
	//       "email": "john@example.com",
	//       "name": "John Doe"
	//     }
	//   }
	// }
}

// ExampleMarshalMultipleResources demonstrates marshaling multiple resources to JSON:API format
func ExampleMarshal_multipleResources() {
	users := []User{
		{ID: "1", Name: "John Doe", Email: "john@example.com"},
		{ID: "2", Name: "Jane Doe", Email: "jane@example.com"},
	}

	data, _ := jsonapi.Marshal(users, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))
	// Output:
	// {
	//   "data": [
	//     {
	//       "id": "1",
	//       "type": "users",
	//       "attributes": {
	//         "email": "john@example.com",
	//         "name": "John Doe"
	//       }
	//     },
	//     {
	//       "id": "2",
	//       "type": "users",
	//       "attributes": {
	//         "email": "jane@example.com",
	//         "name": "Jane Doe"
	//       }
	//     }
	//   ]
	// }
}

// ExampleMarshalRelationships demonstrates marshaling relationships with included resources
func ExampleMarshal_relationships() {
	userWithPosts := UserWithPosts{
		ID:   "1",
		Name: "John Doe",
		Posts: []Post{
			{ID: "1", Title: "First Post", Body: "Content 1"},
			{ID: "2", Title: "Second Post", Body: "Content 2"},
		},
	}

	data, _ := jsonapi.Marshal(userWithPosts,
		jsonapi.IncludeRelatedResources(),
		jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
			return json.MarshalIndent(out, "", "  ")
		}))
	fmt.Println(string(data))
	// Output:
	// {
	//   "data": {
	//     "id": "1",
	//     "type": "users",
	//     "attributes": {
	//       "name": "John Doe"
	//     },
	//     "relationships": {
	//       "posts": {
	//         "data": [
	//           {
	//             "id": "1",
	//             "type": "posts"
	//           },
	//           {
	//             "id": "2",
	//             "type": "posts"
	//           }
	//         ]
	//       }
	//     }
	//   },
	//   "included": [
	//     {
	//       "id": "1",
	//       "type": "posts",
	//       "attributes": {
	//         "body": "Content 1",
	//         "title": "First Post"
	//       }
	//     },
	//     {
	//       "id": "2",
	//       "type": "posts",
	//       "attributes": {
	//         "body": "Content 2",
	//         "title": "Second Post"
	//       }
	//     }
	//   ]
	// }
}

// ExampleMarshalCustom demonstrates custom marshaling using the MarshalJSONAPIResource interface
func ExampleMarshal_custom() {
	customUser := CustomUser{
		ID:   "1",
		Name: "John Doe",
	}

	data, _ := jsonapi.Marshal(customUser, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))
	// Output:
	// {
	//   "data": {
	//     "id": "1",
	//     "type": "users",
	//     "attributes": {
	//       "custom_field": "custom_value",
	//       "name": "John Doe"
	//     }
	//   }
	// }
}

// ExampleUnmarshalSingleResource demonstrates unmarshaling a single resource from JSON:API format
func ExampleUnmarshal_singleResource() {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`

	var unmarshaledUser User
	err := jsonapi.Unmarshal([]byte(jsonData), &unmarshaledUser)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Unmarshaled User: ID=%s, Name=%s, Email=%s\n",
			unmarshaledUser.ID, unmarshaledUser.Name, unmarshaledUser.Email)
	}
	// Output:
	// Unmarshaled User: ID=1, Name=John Doe, Email=john@example.com
}

// ExampleUnmarshalMultipleResources demonstrates unmarshaling multiple resources from JSON:API format
func ExampleUnmarshal_multipleResources() {
	multipleJsonData := `{
		"data": [
			{
				"type": "users",
				"id": "1",
				"attributes": {
					"name": "John Doe",
					"email": "john@example.com"
				}
			},
			{
				"type": "users",
				"id": "2",
				"attributes": {
					"name": "Jane Doe",
					"email": "jane@example.com"
				}
			}
		]
	}`

	var unmarshaledUsers []User
	err := jsonapi.Unmarshal([]byte(multipleJsonData), &unmarshaledUsers)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Number of users: %d\n", len(unmarshaledUsers))
		for i, user := range unmarshaledUsers {
			fmt.Printf("User %d: ID=%s, Name=%s, Email=%s\n",
				i+1, user.ID, user.Name, user.Email)
		}
	}
	// Output:
	// Number of users: 2
	// User 1: ID=1, Name=John Doe, Email=john@example.com
	// User 2: ID=2, Name=Jane Doe, Email=jane@example.com
}

// ExampleUnmarshalRelationships demonstrates unmarshaling relationships with included resources
func ExampleUnmarshal_relationships() {
	relationshipJsonData := `{
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

	var userWithPostsUnmarshaled UserWithPosts
	err := jsonapi.Unmarshal([]byte(relationshipJsonData), &userWithPostsUnmarshaled, jsonapi.PopulateFromIncluded())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User: ID=%s, Name=%s\n", userWithPostsUnmarshaled.ID, userWithPostsUnmarshaled.Name)
		fmt.Printf("Number of posts: %d\n", len(userWithPostsUnmarshaled.Posts))
		for i, post := range userWithPostsUnmarshaled.Posts {
			fmt.Printf("Post %d: ID=%s, Title=%s, Body=%s\n",
				i+1, post.ID, post.Title, post.Body)
		}
	}
	// Output:
	// User: ID=1, Name=John Doe
	// Number of posts: 2
	// Post 1: ID=1, Title=First Post, Body=Content 1
	// Post 2: ID=2, Title=Second Post, Body=Content 2
}

// ExampleUnmarshalCustom demonstrates custom unmarshaling using the UnmarshalJSONAPIResource interface
func ExampleUnmarshal_custom() {
	customJsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe",
				"custom_field": "custom_value"
			}
		}
	}`

	var customUnmarshaledUser CustomUnmarshalUser
	err := jsonapi.Unmarshal([]byte(customJsonData), &customUnmarshaledUser)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Custom User: ID=%s, Name=%s, CustomField=%s\n",
			customUnmarshaledUser.ID, customUnmarshaledUser.Name, customUnmarshaledUser.CustomField)
	}
	// Output:
	// Custom User: ID=1, Name=John Doe, CustomField=custom_value
}

// ExampleUnmarshalNestedStruct demonstrates unmarshaling nested struct attributes
func ExampleUnmarshal_nestedStruct() {
	nestedJsonData := `{
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

	var userWithAddressUnmarshaled UserWithAddress
	err := jsonapi.Unmarshal([]byte(nestedJsonData), &userWithAddressUnmarshaled)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User: ID=%s, Name=%s\n", userWithAddressUnmarshaled.ID, userWithAddressUnmarshaled.Name)
		fmt.Printf("Address: Street=%s, City=%s\n",
			userWithAddressUnmarshaled.Address.Street, userWithAddressUnmarshaled.Address.City)
	}
	// Output:
	// User: ID=1, Name=John Doe
	// Address: Street=123 Main St, City=Anytown
}

// ExampleUnmarshalDocument demonstrates unmarshaling a JSON:API document directly
func ExampleUnmarshalDocument() {
	jsonData := `{
		"data": {
			"type": "users",
			"id": "1",
			"attributes": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}
	}`

	doc, err := jsonapi.UnmarshalDocument([]byte(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		resource, ok := doc.Data.One()
		if ok {
			fmt.Printf("Document Resource: ID=%s, Type=%s\n", resource.ID, resource.Type)
			fmt.Printf("Name: %s\n", resource.Attributes["name"])
			fmt.Printf("Email: %s\n", resource.Attributes["email"])
		}
	}
	// Output:
	// Document Resource: ID=1, Type=users
	// Name: John Doe
	// Email: john@example.com
}

// Example demonstrates marshaling and then unmarshaling (round-trip)
func Example() {
	originalUser := User{
		ID:    "42",
		Name:  "Alice Smith",
		Email: "alice@example.com",
	}

	// Marshal to JSON:API
	marshaledData, _ := jsonapi.Marshal(originalUser)

	// Unmarshal back to struct
	var roundTripUser User
	err := jsonapi.Unmarshal(marshaledData, &roundTripUser)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Round-trip User: ID=%s, Name=%s, Email=%s\n",
			roundTripUser.ID, roundTripUser.Name, roundTripUser.Email)
		fmt.Printf("Round-trip successful: %t\n",
			originalUser.ID == roundTripUser.ID &&
				originalUser.Name == roundTripUser.Name &&
				originalUser.Email == roundTripUser.Email)
	}
	// Output:
	// Round-trip User: ID=42, Name=Alice Smith, Email=alice@example.com
	// Round-trip successful: true
}
