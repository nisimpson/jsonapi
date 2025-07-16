package main

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

func main() {
	fmt.Println("JSON:API Library Examples")
	fmt.Println("========================")

	// MARSHALING EXAMPLES
	fmt.Println("\n=== MARSHALING EXAMPLES ===")

	// Example 1: Single resource
	fmt.Println("\n1. Single Resource Marshaling:")
	user := User{
		ID:    "1",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	data, _ := jsonapi.Marshal(user, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))

	// Example 2: Multiple resources
	fmt.Println("\n2. Multiple Resources Marshaling:")
	users := []User{
		{ID: "1", Name: "John Doe", Email: "john@example.com"},
		{ID: "2", Name: "Jane Doe", Email: "jane@example.com"},
	}

	data, _ = jsonapi.Marshal(users, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))

	// Example 3: Relationships with included resources
	fmt.Println("\n3. Relationships with Included Resources:")
	userWithPosts := UserWithPosts{
		ID:   "1",
		Name: "John Doe",
		Posts: []Post{
			{ID: "1", Title: "First Post", Body: "Content 1"},
			{ID: "2", Title: "Second Post", Body: "Content 2"},
		},
	}

	data, _ = jsonapi.Marshal(userWithPosts,
		jsonapi.IncludeRelatedResources(),
		jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
			return json.MarshalIndent(out, "", "  ")
		}))
	fmt.Println(string(data))

	// Example 4: Custom marshaling
	fmt.Println("\n4. Custom Marshaling:")
	customUser := CustomUser{
		ID:   "1",
		Name: "John Doe",
	}

	data, _ = jsonapi.Marshal(customUser, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
		return json.MarshalIndent(out, "", "  ")
	}))
	fmt.Println(string(data))

	// UNMARSHALING EXAMPLES
	fmt.Println("\n=== UNMARSHALING EXAMPLES ===")

	// Example 5: Single resource unmarshaling
	fmt.Println("\n5. Single Resource Unmarshaling:")
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
		fmt.Printf("Unmarshaled User: %+v\n", unmarshaledUser)
	}

	// Example 6: Multiple resources unmarshaling
	fmt.Println("\n6. Multiple Resources Unmarshaling:")
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
	err = jsonapi.Unmarshal([]byte(multipleJsonData), &unmarshaledUsers)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Unmarshaled Users: %+v\n", unmarshaledUsers)
	}

	// Example 7: Relationships with included resources unmarshaling
	fmt.Println("\n7. Relationships with Included Resources Unmarshaling:")
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
	err = jsonapi.Unmarshal([]byte(relationshipJsonData), &userWithPostsUnmarshaled, jsonapi.PopulateFromIncluded())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User with Posts: %+v\n", userWithPostsUnmarshaled)
		for i, post := range userWithPostsUnmarshaled.Posts {
			fmt.Printf("  Post %d: %+v\n", i+1, post)
		}
	}

	// Example 8: Custom unmarshaling
	fmt.Println("\n8. Custom Unmarshaling:")
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
	err = jsonapi.Unmarshal([]byte(customJsonData), &customUnmarshaledUser)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Custom Unmarshaled User: %+v\n", customUnmarshaledUser)
	}

	// Example 9: Nested struct attributes unmarshaling
	fmt.Println("\n9. Nested Struct Attributes Unmarshaling:")
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
	err = jsonapi.Unmarshal([]byte(nestedJsonData), &userWithAddressUnmarshaled)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User with Address: %+v\n", userWithAddressUnmarshaled)
	}

	// Example 10: Document unmarshaling
	fmt.Println("\n10. Document Unmarshaling:")
	doc, err := jsonapi.UnmarshalDocument([]byte(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		resource, ok := doc.Data.One()
		if ok {
			fmt.Printf("Document Resource: ID=%s, Type=%s, Attributes=%+v\n", 
				resource.ID, resource.Type, resource.Attributes)
		}
	}

	// BIDIRECTIONAL EXAMPLE
	fmt.Println("\n=== BIDIRECTIONAL EXAMPLE ===")
	fmt.Println("\n11. Marshal then Unmarshal (Round-trip):")
	
	originalUser := User{
		ID:    "42",
		Name:  "Alice Smith",
		Email: "alice@example.com",
	}
	
	// Marshal to JSON:API
	marshaledData, _ := jsonapi.Marshal(originalUser)
	fmt.Printf("Marshaled: %s\n", string(marshaledData))
	
	// Unmarshal back to struct
	var roundTripUser User
	err = jsonapi.Unmarshal(marshaledData, &roundTripUser)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Round-trip User: %+v\n", roundTripUser)
		fmt.Printf("Round-trip successful: %t\n", originalUser == roundTripUser)
	}
}
