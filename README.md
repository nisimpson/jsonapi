# JSON:API Library for Go

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
- ✅ 100% test coverage with comprehensive edge case validation

## Installation

```bash
go get github.com/nisimpson/jsonapi
```

## Quick Start

### Marshaling (Go → JSON:API)

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/nisimpson/jsonapi"
)

type User struct {
    ID    string `jsonapi:"primary,users"`
    Name  string `jsonapi:"attr,name"`
    Email string `jsonapi:"attr,email"`
}

func main() {
    user := User{
        ID:    "1",
        Name:  "John Doe",
        Email: "john@example.com",
    }

    data, err := jsonapi.Marshal(user)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(data))
}
```

Output:
```json
{
  "data": {
    "type": "users",
    "id": "1",
    "attributes": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  }
}
```

### Unmarshaling (JSON:API → Go)

```go
package main

import (
    "fmt"
    "github.com/nisimpson/jsonapi"
)

type User struct {
    ID    string `jsonapi:"primary,users"`
    Name  string `jsonapi:"attr,name"`
    Email string `jsonapi:"attr,email"`
}

func main() {
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

    var user User
    err := jsonapi.Unmarshal([]byte(jsonData), &user)
    if err != nil {
        panic(err)
    }

    fmt.Printf("User: %+v\n", user)
    // Output: User: {ID:1 Name:John Doe Email:john@example.com}
}
```

## Core Types

### Document
The top-level JSON:API document structure containing data, meta, links, errors, and included resources.

### Resource
Represents a JSON:API resource object with ID, type, attributes, relationships, links, and meta.

### PrimaryData
Represents the primary data which can be a single resource, multiple resources, or null.

### Relationship
Represents a JSON:API relationship object with data, links, and meta.

## Struct Tags

Use the `jsonapi` struct tag to define how fields should be marshaled:

### Primary Key
```go
type User struct {
    ID string `jsonapi:"primary,users"`
}
```

### Attributes
```go
type User struct {
    Name  string `jsonapi:"attr,name"`
    Email string `jsonapi:"attr,email,omitempty"`
}
```

### Relationships
```go
type User struct {
    Posts []Post `jsonapi:"relation,posts"`
}
```

## Advanced Usage

### Relationships with Included Resources

```go
type User struct {
    ID    string `jsonapi:"primary,users"`
    Name  string `jsonapi:"attr,name"`
    Posts []Post `jsonapi:"relation,posts"`
}

type Post struct {
    ID    string `jsonapi:"primary,posts"`
    Title string `jsonapi:"attr,title"`
}

user := User{
    ID:   "1",
    Name: "John Doe",
    Posts: []Post{
        {ID: "1", Title: "First Post"},
        {ID: "2", Title: "Second Post"},
    },
}

data, err := jsonapi.Marshal(user, jsonapi.IncludeRelatedResources())
```

### Embedded Structs

```go
type Timestamp struct {
    CreatedAt time.Time `jsonapi:"attr,created_at"`
    UpdatedAt time.Time `jsonapi:"attr,updated_at"`
}

type User struct {
    Timestamp  // Embedded struct
    ID   string `jsonapi:"primary,users"`
    Name string `jsonapi:"attr,name"`
}
```

### Custom Marshaling

Implement the `ResourceMarshaler` interface for custom resource marshaling:

```go
type User struct {
    ID   string
    Name string
}

func (u User) MarshalJSONAPIResource(ctx context.Context) (jsonapi.Resource, error) {
    return jsonapi.Resource{
        Type: "users",
        ID:   u.ID,
        Attributes: map[string]interface{}{
            "name": u.Name,
            "custom_field": "custom_value",
        },
    }, nil
}
```

### Custom Links and Meta

```go
func (u User) MarshalJSONAPILinks(ctx context.Context) (map[string]jsonapi.Link, error) {
    return map[string]jsonapi.Link{
        "self": {Href: "/users/" + u.ID},
    }, nil
}

func (u User) MarshalJSONAPIMeta(ctx context.Context) (map[string]interface{}, error) {
    return map[string]interface{}{
        "version": "1.0",
    }, nil
}
```

## Document Manipulation

### MarshalDocument

For cases where you need to access or modify the Document structure before serialization, you can use the MarshalDocument function:

```go
// Get the Document structure for further manipulation
doc, err := jsonapi.MarshalDocument(context.Background(), user)
if err != nil {
    panic(err)
}

// Add custom meta information
doc.Meta = map[string]interface{}{
    "version": "1.0",
}

// Then marshal the document
data, err = json.MarshalIndent(doc, "", "  ")
fmt.Println(string(data))
```

This is particularly useful when you need to:
- Add top-level meta information
- Customize the included resources
- Add top-level links
- Inspect the document structure before serialization

## Marshaling Options

### WithMarshaler
Use a custom JSON marshaler:

```go
data, err := jsonapi.Marshal(user, jsonapi.WithMarshaler(func(out interface{}) ([]byte, error) {
    return json.MarshalIndent(out, "", "  ")
}))
```

### IncludeRelatedResources
Include related resources in the `included` array:

```go
data, err := jsonapi.Marshal(user, jsonapi.IncludeRelatedResources())
```

## Unmarshaling Options

### WithUnmarshaler
Use a custom JSON unmarshaler:

```go
err := jsonapi.Unmarshal(data, &user, jsonapi.WithUnmarshaler(func(data []byte, out interface{}) error {
    return json.Unmarshal(data, out)
}))
```

### StrictMode
Enable strict validation during unmarshaling:

```go
err := jsonapi.Unmarshal(data, &user, jsonapi.StrictMode())
```

### PopulateFromIncluded
Populate relationship fields from included resources:

```go
err := jsonapi.Unmarshal(data, &user, jsonapi.PopulateFromIncluded())
```

## Context Support

All marshaling and unmarshaling operations support Go contexts:

```go
ctx := context.WithTimeout(context.Background(), 5*time.Second)
data, err := jsonapi.MarshalWithContext(ctx, user)
```

```go
ctx := context.WithTimeout(context.Background(), 5*time.Second)
err := jsonapi.UnmarshalWithContext(ctx, data, &user)
```

## Server Package

The server package provides comprehensive utilities for building JSON:API compliant HTTP handlers with automatic request processing and response generation.

### Key Components

- **RequestContext**: Extracts and validates JSON:API request parameters
- **ResourceHandler**: Interface for handling resource CRUD operations
- **ResourceHandlerMux**: HTTP multiplexer for routing requests by resource type
- **RelationshipHandler**: Interface for handling relationship operations
- **Response utilities**: Functions for writing JSON:API compliant responses

### Request Parameter Parsing

The server package automatically parses JSON:API query parameters:

```go
import "github.com/nisimpson/jsonapi/server"

func handleUsers(w http.ResponseWriter, r *http.Request) {
    ctx, err := server.NewRequestContext(r)
    if err != nil {
        server.WriteError(w, jsonapi.Error{
            Status: "400",
            Title:  "Bad Request",
            Detail: err.Error(),
        })
        return
    }

    // Sparse fieldsets: ?fields[users]=name,email
    userFields := ctx.GetFields("users")
    
    // Includes: ?include=posts,comments
    if ctx.ShouldInclude("posts") {
        // Include related posts
    }
    
    // Sorting: ?sort=name,-created_at
    sortFields := ctx.Sort
    
    // Pagination: ?page[number]=1&page[size]=10
    pageNumber := ctx.GetPageParam("number")
    pageSize := ctx.GetPageParam("size")
    
    // Filtering: ?filter[status]=active
    statusFilter := ctx.GetFilterParam("status")
    
    // Process request and return response
    users := getUsersWithParams(userFields, statusFilter, pageNumber, pageSize)
    server.WriteResponse(w, users, http.StatusOK)
}
```

### Resource Handler Interface

Implement the ResourceHandler interface for complete CRUD operations:

```go
type UserHandler struct {
    users map[string]*User
    mu    sync.RWMutex
}

func (h *UserHandler) GetResource(ctx context.Context, id string) (interface{}, error) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    user, exists := h.users[id]
    if !exists {
        return nil, jsonapi.Error{
            Status: "404",
            Title:  "Not Found",
            Detail: "User not found",
        }
    }
    return user, nil
}

func (h *UserHandler) GetResources(ctx context.Context) (interface{}, error) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    users := make([]*User, 0, len(h.users))
    for _, user := range h.users {
        users = append(users, user)
    }
    return users, nil
}

func (h *UserHandler) CreateResource(ctx context.Context, resource interface{}) (interface{}, error) {
    user, ok := resource.(*User)
    if !ok {
        return nil, jsonapi.Error{
            Status: "400",
            Title:  "Bad Request",
            Detail: "Invalid resource type",
        }
    }
    
    h.mu.Lock()
    defer h.mu.Unlock()
    
    user.ID = generateID()
    h.users[user.ID] = user
    return user, nil
}

func (h *UserHandler) UpdateResource(ctx context.Context, id string, resource interface{}) (interface{}, error) {
    user, ok := resource.(*User)
    if !ok {
        return nil, jsonapi.Error{
            Status: "400",
            Title:  "Bad Request",
            Detail: "Invalid resource type",
        }
    }
    
    h.mu.Lock()
    defer h.mu.Unlock()
    
    if _, exists := h.users[id]; !exists {
        return nil, jsonapi.Error{
            Status: "404",
            Title:  "Not Found",
            Detail: "User not found",
        }
    }
    
    user.ID = id
    h.users[id] = user
    return user, nil
}

func (h *UserHandler) DeleteResource(ctx context.Context, id string) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    if _, exists := h.users[id]; !exists {
        return jsonapi.Error{
            Status: "404",
            Title:  "Not Found",
            Detail: "User not found",
        }
    }
    
    delete(h.users, id)
    return nil
}
```

### HTTP Server Setup

Use the ResourceHandlerMux for automatic routing:

```go
func main() {
    mux := server.NewResourceHandlerMux()
    
    // Register handlers for different resource types
    mux.Handle("users", &UserHandler{users: make(map[string]*User)})
    mux.Handle("posts", &PostHandler{posts: make(map[string]*Post)})
    
    // The mux automatically handles:
    // GET /users -> GetResources()
    // GET /users/123 -> GetResource("123")
    // POST /users -> CreateResource()
    // PATCH /users/123 -> UpdateResource("123")
    // DELETE /users/123 -> DeleteResource("123")
    
    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", mux)
}
```

### Error Handling

The server package provides JSON:API compliant error responses:

```go
func handleValidationError(w http.ResponseWriter, r *http.Request) {
    errors := []jsonapi.Error{
        {
            Status: "422",
            Code:   "VALIDATION_ERROR",
            Title:  "Validation Failed",
            Detail: "Name is required",
            Source: map[string]interface{}{
                "pointer": "/data/attributes/name",
            },
        },
        {
            Status: "422",
            Code:   "VALIDATION_ERROR", 
            Title:  "Validation Failed",
            Detail: "Email must be valid",
            Source: map[string]interface{}{
                "pointer": "/data/attributes/email",
            },
        },
    }

    server.WriteError(w, errors...)
}
```

### Content Negotiation

The server package automatically handles JSON:API content negotiation:

- Validates `application/vnd.api+json` Content-Type for requests
- Sets proper `application/vnd.api+json` Content-Type for responses
- Returns 415 Unsupported Media Type for invalid Content-Type
- Returns 406 Not Acceptable for invalid Accept headers

### Response Writing

Use the response utilities for consistent JSON:API responses:

```go
// Write a successful resource response
server.WriteResponse(w, user, http.StatusOK)

// Write a collection response
server.WriteResponse(w, users, http.StatusOK, jsonapi.IncludeRelatedResources())

// Write an error response
server.WriteError(w, jsonapi.Error{
    Status: "500",
    Title:  "Internal Server Error",
    Detail: "Something went wrong",
})

// Write a custom document
doc := &jsonapi.Document{
    Data: jsonapi.SingleResource(resource),
    Meta: map[string]interface{}{
        "total": 100,
    },
}
server.WriteDocument(w, doc, http.StatusOK)
```

## Error Handling

The library provides detailed error messages for common issues:

- Nil pointer inputs
- Non-struct types
- JSON marshaling/unmarshaling failures
- Custom marshaler/unmarshaler errors
- Invalid JSON:API document structure
- Type conversion failures
- Resource type mismatches

## Thread Safety

The library is thread-safe and can be used concurrently without additional synchronization.

## Testing

The library includes comprehensive test coverage with 100% statement coverage across all packages.

Run the test suite:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

Run tests with detailed coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Coverage

- **Core Package**: 100% statement coverage with comprehensive edge case testing
- **Server Package**: 100% statement coverage with HTTP handler integration tests
- **Unmarshaling**: Complete test suite covering all unmarshaling scenarios
- **Error Handling**: Extensive error condition validation
- **Concurrent Operations**: Thread-safety validation across all components

## Examples

See the `example.go` file for comprehensive usage examples.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## JSON:API Specification

This library implements the [JSON:API v1.0 specification](https://jsonapi.org/format/).
