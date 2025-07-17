# JSON:API Library for Go

[![Test](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml/badge.svg)](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/nisimpson/jsonapi?status.svg)](http://godoc.org/github.com/nisimpson/jsonapi)
[![Release](https://img.shields.io/github/release/nisimpson/jsonapi.svg)](https://github.com/nisimpson/jsonapi/releases)

A comprehensive Go library for marshaling Go structs into JSON:API compliant resources and unmarshaling JSON:API documents back into Go structs. This library supports both automatic struct tag-based marshaling/unmarshaling and custom marshaling/unmarshaling through interfaces.

## Features

- ✅ Full JSON:API specification compliance (marshaling & unmarshaling)
- ✅ Struct tag-based automatic marshaling/unmarshaling
- ✅ Custom marshaling/unmarshaling interfaces
- ✅ Relationship handling with included resources
- ✅ Embedded struct support
- ✅ Context support for all operations
- ✅ Thread-safe operations
- ✅ Comprehensive error handling
- ✅ Zero-value and omitempty support
- ✅ Strict mode validation for unmarshaling
- ✅ Type conversion during unmarshaling
- ✅ HTTP server utilities for JSON:API endpoints
- ✅ Request parameter parsing (sparse fieldsets, includes, sorting, pagination, filtering)
- ✅ Content negotiation and proper header handling
- ✅ Resource and relationship handlers for HTTP servers
- ✅ Default HTTP routing for JSON:API endpoints
- ✅ Iterator support for resource collections
- ✅ 100% test coverage with comprehensive edge case validation

## Installation

```bash
go get github.com/nisimpson/jsonapi
```

### Requirements

- Go 1.24.4 or higher

## Basic Usage

### Marshaling

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/nisimpson/jsonapi"
)

// Define a struct with jsonapi tags
type User struct {
    ID    string `jsonapi:"primary,users"`
    Name  string `jsonapi:"attr,name"`
    Email string `jsonapi:"attr,email,omitempty"`
}

func main() {
    user := User{
        ID:    "123",
        Name:  "John Doe",
        Email: "john@example.com",
    }

    // Marshal to JSON:API
    data, err := jsonapi.Marshal(user)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(data))
    // Output:
    // {"data":{"id":"123","type":"users","attributes":{"email":"john@example.com","name":"John Doe"}}}
}
```

### Unmarshaling

```go
package main

import (
    "context"
    "fmt"

    "github.com/nisimpson/jsonapi"
)

// Define a struct with jsonapi tags
type User struct {
    ID    string `jsonapi:"primary,users"`
    Name  string `jsonapi:"attr,name"`
    Email string `jsonapi:"attr,email,omitempty"`
}

func main() {
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

    var user User
    err := jsonapi.Unmarshal(jsonData, &user)
    if err != nil {
        panic(err)
    }

    fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
    // Output:
    // User: John Doe (john@example.com)
}
```

## Struct Tags

The library uses struct tags to determine how to marshal and unmarshal JSON:API resources:

```go
type User struct {
    ID       string `jsonapi:"primary,users"`        // Primary resource ID and type
    Name     string `jsonapi:"attr,name"`            // Attribute
    Email    string `jsonapi:"attr,email,omitempty"` // Optional attribute
    Posts    []Post `jsonapi:"relation,posts"`       // To-many relationship
    Profile  Profile `jsonapi:"relation,profile"`    // To-one relationship
    Metadata string `jsonapi:"-"`                    // Ignored field
}
```

### Tag Format

- `primary,type`: Marks a field as the primary ID field and specifies the resource type
- `attr,name[,omitempty]`: Marks a field as an attribute with optional omitempty flag
- `relation,name[,omitempty]`: Marks a field as a relationship with optional omitempty flag
- `-`: Ignores the field during marshaling/unmarshaling

## Relationships

The library supports both to-one and to-many relationships:

```go
type User struct {
    ID      string `jsonapi:"primary,users"`
    Name    string `jsonapi:"attr,name"`
    Profile Profile `jsonapi:"relation,profile"` // To-one relationship
    Posts   []Post  `jsonapi:"relation,posts"`   // To-many relationship
}

type Profile struct {
    ID       string `jsonapi:"primary,profiles"`
    Bio      string `jsonapi:"attr,bio"`
    UserID   string `jsonapi:"attr,user_id"`
}

type Post struct {
    ID      string `jsonapi:"primary,posts"`
    Title   string `jsonapi:"attr,title"`
    Content string `jsonapi:"attr,content"`
    UserID  string `jsonapi:"attr,user_id"`
}
```

### Including Related Resources

You can include related resources in the response:

```go
// Marshal with included related resources
data, err := jsonapi.Marshal(user, jsonapi.IncludeRelatedResources())
```

## Custom Marshaling/Unmarshaling

The library supports custom marshaling and unmarshaling through interfaces:

```go
// Custom resource marshaling
type ResourceMarshaler interface {
    MarshalJSONAPIResource(ctx context.Context) (Resource, error)
}

// Custom resource unmarshaling
type ResourceUnmarshaler interface {
    UnmarshalJSONAPIResource(ctx context.Context, resource Resource) error
}
```

Other interfaces are available for more granular control:

```go
// Marshaling interfaces
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

// Unmarshaling interfaces
type LinksUnmarshaler interface {
    UnmarshalJSONAPILinks(ctx context.Context, links map[string]Link) error
}

type MetaUnmarshaler interface {
    UnmarshalJSONAPIMeta(ctx context.Context, meta map[string]interface{}) error
}

type RelationshipLinksUnmarshaler interface {
    UnmarshalJSONAPIRelationshipLinks(ctx context.Context, name string, links map[string]Link) error
}

type RelationshipMetaUnmarshaler interface {
    UnmarshalJSONAPIRelationshipMeta(ctx context.Context, name string, meta map[string]interface{}) error
}
```

## Iterator Support

The library provides iterator support for resource collections using Go's `iter` package:

```go
// Get a document with multiple resources
doc, err := jsonapi.MarshalDocument(context.Background(), users)
if err != nil {
    panic(err)
}

// Iterate over resources in the primary data
for resource := range doc.Data.Iter() {
    fmt.Printf("Resource ID: %s, Type: %s\n", resource.ID, resource.Type)

    // Process attributes
    for name, value := range resource.Attributes {
        fmt.Printf("Attribute %s: %v\n", name, value)
    }

    // Process relationships
    for name, rel := range resource.Relationships {
        fmt.Printf("Relationship %s\n", name)
    }
}
```

This makes it easy to process large collections of resources efficiently without having to manually check if the primary data contains a single resource or multiple resources.

## HTTP Server Support

The library includes a `server` package that provides HTTP server utilities for building JSON:API compliant web services. It includes request context management, resource handlers, and routing utilities that simplify the creation of JSON:API endpoints following the specification.

### Resource Handlers

The `ResourceHandler` type provides HTTP handlers for different JSON:API resource operations:

```go
type ResourceHandler struct {
    Get          http.Handler // Handler for GET requests to retrieve a single resource
    Create       http.Handler // Handler for POST requests to create new resources
    Update       http.Handler // Handler for PATCH requests to update existing resources
    Delete       http.Handler // Handler for DELETE requests to remove resources
    Search       http.Handler // Handler for GET requests to search/list resources
    Relationship http.Handler // Handler for relationship-specific operations
}
```

### Relationship Handlers

The `RelationshipHandler` type provides HTTP handlers for JSON:API relationship operations:

```go
type RelationshipHandler struct {
    Get    http.Handler // Handler for GET requests to fetch relationship linkage
    Add    http.Handler // Handler for POST requests to add to to-many relationships
    Update http.Handler // Handler for PATCH requests to update relationship linkage
    Delete http.Handler // Handler for DELETE requests to remove from to-many relationships
}
```

### Default HTTP Routing

The `DefaultHandler` function creates a default HTTP handler with standard JSON:API routes configured:

```go
func DefaultHandler(mux ResourceHandlerMux) http.Handler {
    // Sets up all the conventional JSON:API endpoints including:
    // - "GET    /{type}"                                   // Search/list resources of a type
    // - "GET    /{type}/{id}"                              // Get a single resource by ID
    // - "POST   /{type}"                                   // Create a new resource
    // - "PATCH  /{type}/{id}"                              // Update an existing resource
    // - "DELETE /{type}/{id}"                              // Delete a resource
    // - "GET    /{type}/{id}/relationships/{relationship}" // Get a resource's relationship
    // - "GET    /{type}/{id}/{related}"                    // Get related resources
    // - "POST   /{type}/{id}/relationships/{relationship}" // Add to a to-many relationship
    // - "PATCH  /{type}/{id}/relationships/{relationship}" // Update a relationship
    // - "DELETE /{type}/{id}/relationships/{relationship}" // Remove from a to-many relationship
}
```

### Request Context

The `RequestContext` type contains parsed information from an HTTP request that is relevant to JSON:API resource operations:

```go
type RequestContext struct {
    ResourceID            string // The ID of the requested resource
    ResourceType          string // The type of the requested resource
    Relationship          string // The name of the requested relationship
    FetchRelatedResources bool   // Whether to fetch related resources instead of relationship linkage
}
```

### Example Server Setup

```go
package main

import (
    "net/http"

    "github.com/nisimpson/jsonapi"
    "github.com/nisimpson/jsonapi/server"
)

func main() {
    // Create resource handlers
    usersHandler := server.ResourceHandler{
        Get: http.HandlerFunc(getUserHandler),
        Create: http.HandlerFunc(createUserHandler),
        Search: http.HandlerFunc(searchUsersHandler),
        // Add other handlers as needed
    }

    // Create a resource handler mux
    mux := server.ResourceHandlerMux{
        "users": usersHandler,
        // Add other resource types as needed
    }

    // Create a default handler with standard JSON:API routes
    handler := server.DefaultHandler(mux)

    // Start the server
    http.ListenAndServe(":8080", handler)
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    requestContext, _ := server.GetRequestContext(ctx)

    // Get the user by ID
    user := getUser(requestContext.ResourceID)

    // Marshal the user to JSON:API
    doc, err := jsonapi.MarshalDocument(ctx, user)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Write the response
    w.Header().Set("Content-Type", "application/vnd.api+json")
    json.NewEncoder(w).Encode(doc)
}

// Implement other handlers similarly
```

### Using server.Response and server.HandlerFunc

The library provides a more convenient way to write JSON:API handlers using `server.HandlerFunc` and `server.Response`:

```go
package main

import (
    "net/http"

    "github.com/nisimpson/jsonapi"
    "github.com/nisimpson/jsonapi/server"
)

func main() {
    // Create resource handlers using HandlerFunc
    usersHandler := server.ResourceHandler{
        Get:    server.HandlerFunc(getUser),
        Create: server.HandlerFunc(createUser),
        Search: server.HandlerFunc(searchUsers),
    }

    // Create a resource handler mux
    mux := server.ResourceHandlerMux{
        "users": usersHandler,
    }

    // Create a default handler with standard JSON:API routes
    handler := server.DefaultHandler(mux)

    // Start the server
    http.ListenAndServe(":8080", handler)
}

// Using HandlerFunc for cleaner handler implementation
func getUser(ctx *server.RequestContext, r *http.Request) server.Response {
    // Get the user by ID
    user, err := fetchUserFromDatabase(ctx.ResourceID)
    if err != nil {
        // Return a 404 response with error
        return server.Response{
            Status: http.StatusNotFound,
            Body: jsonapi.NewErrorDocument(jsonapi.Error{
                Status: "404",
                Title:  "Resource not found",
                Detail: err.Error(),
            }),
        }
    }

    // Marshal the user to JSON:API
    doc, err := jsonapi.MarshalDocument(r.Context(), user)
    if err != nil {
        // Return a 500 response with error
        return server.Response{
            Status: http.StatusInternalServerError,
            Body: jsonapi.NewErrorDocument(jsonapi.Error{
                Status: "500",
                Title:  "Internal server error",
                Detail: err.Error(),
            }),
        }
    }

    // Return a structured response
    return server.Response{
        Status: http.StatusOK,
        Header: http.Header{
            "Cache-Control": []string{"max-age=3600"},
        },
        Body: doc,
    }
}

// Example of handling errors with HandlerFunc
func createUser(ctx *server.RequestContext, r *http.Request) server.Response {
    var user User

    // Parse request body
    if err := jsonapi.UnmarshalResourceInto(r.Context(), doc.Data, &user); err != nil {
        return server.Response{
            Status: http.StatusBadRequest,
            Body: jsonapi.NewErrorDocument(jsonapi.Error{
                Status: "400",
                Title:  "Invalid request body",
                Detail: err.Error(),
            }),
        }
    }

    // Save user to database
    if err := saveUserToDatabase(&user); err != nil {
        return server.Response{
            Status: http.StatusInternalServerError,
            Body: jsonapi.NewErrorDocument(jsonapi.Error{
                Status: "500",
                Title:  "Internal server error",
                Detail: err.Error(),
            }),
        }
    }

    // Marshal the created user to JSON:API
    responseDoc, err := jsonapi.MarshalDocument(r.Context(), user)
    if err != nil {
        return server.Response{
            Status: http.StatusInternalServerError,
            Body: jsonapi.NewErrorDocument(jsonapi.Error{
                Status: "500",
                Title:  "Internal server error",
                Detail: err.Error(),
            }),
        }
    }

    // Return a structured response with 201 Created status
    return server.Response{
        Status: http.StatusCreated,
        Header: http.Header{
            "Location": []string{"/users/" + user.ID},
        },
        Body: responseDoc,
    }
}
```

The `server.HandlerFunc` type provides several advantages:

1. Automatic access to the parsed request context
2. Structured response handling with status codes and headers
3. Automatic error handling with proper JSON:API error formatting
4. Cleaner handler implementation with less boilerplate code

The `server.Response` struct allows you to specify:

- HTTP status code
- Custom HTTP headers
- JSON:API document body

The `server.Write` and `server.Error` functions are also available for more direct control over response writing:

```go
// Write a JSON:API document response
server.Write(w, doc, http.StatusOK)

// Write a JSON:API error response
server.Error(w, err, http.StatusBadRequest)
```

## Request Parameter Parsing

The `RequestContext` provides methods for parsing JSON:API query parameters:

### Sparse Fieldsets

```go
// Get sparse fieldsets for a specific resource type
fields := requestContext.GetFields(r, "users")
// fields = ["name", "email"] for ?fields[users]=name,email
```

### Includes

```go
// Check if a relationship should be included
shouldIncludePosts := requestContext.ShouldInclude(r, "posts")
// true for ?include=posts,comments
```

### Content Negotiation

The server package provides middleware for proper JSON:API content negotiation:

```go
// Use content negotiation middleware
handler = server.UseContentNegotiation(handler)
```

This ensures proper handling of the `Accept` and `Content-Type` headers according to the JSON:API specification.

## Error Handling

The library provides comprehensive error handling with detailed error messages:

```go
// Create an error document
errorDoc := jsonapi.Document{
    Errors: []jsonapi.Error{
        {
            Status: "404",
            Title:  "Resource not found",
            Detail: "The requested resource could not be found",
        },
    },
}

// Marshal the error document
data, err := json.Marshal(errorDoc)
if err != nil {
    panic(err)
}

// Write the error response
w.Header().Set("Content-Type", "application/vnd.api+json")
w.WriteHeader(http.StatusNotFound)
w.Write(data)
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
