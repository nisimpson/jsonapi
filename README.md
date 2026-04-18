# JSON:API Library for Go

[![Test](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml/badge.svg)](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/nisimpson/jsonapi?status.svg)](http://godoc.org/github.com/nisimpson/jsonapi)
[![Release](https://img.shields.io/github/release/nisimpson/jsonapi.svg)](https://github.com/nisimpson/jsonapi/releases)

A comprehensive Go library for building JSON:API compliant REST APIs. This library provides marshaling, unmarshaling, and HTTP handling utilities that follow the [JSON:API specification](https://jsonapi.org/).

## Features

- **Complete JSON:API Support**: Resources, relationships, links, meta, errors, and includes
- **Type-Safe Marshaling**: Struct-based resource definitions with interface-driven customization
- **Flexible Unmarshaling**: Support for creation, updates, and relationship operations
- **HTTP Server Integration**: Built-in mux with automatic request parsing and context injection
- **HTTP Client**: Typed client for consuming JSON:API servers with CRUD, relationships, pagination, and middleware
- **Link Resolution**: Pluggable URL generation without server awareness in resources
- **Validation**: Optional type validation and comprehensive error handling
- **Performance**: Efficient marshaling with configurable include depth limits

## Installation

```bash
go get github.com/nisimpson/jsonapi/v2
```

## Quick Start

### Define Resources

```go
type Article struct {
    ID       string `json:"-"`
    Title    string `json:"title"`
    Content  string `json:"content"`
    Author   *User  `json:"author"`
    Tags     []Tag  `json:"-"`
}

func (a Article) ResourceType() string { return "articles" }
func (a Article) ResourceID() string { return a.ID }

// Implement relationships
func (a Article) Relationships() map[string]jsonapi.RelationType {
	return map[string]jsonapi.RelationType{
		"author":   jsonapi.RelationToOne,     // Single related resource
		"tags":     jsonapi.RelationToMany,    // Multiple related resources
		"comments": jsonapi.RelationLinksOnly, // Links only, no data included
	}
}

func (a Article) MarshalRef(name string) []jsonapi.ResourceIdentifier {
	switch name {
	case "author":
		return jsonapi.OneRef(a.User)
	case "tags":
		return jsonapi.ManyRef(a.Tags...)
	}
	return nil
}

func (a *Article) UnmarshalRef(name, id string, meta map[string]interface{}) error {
    switch name {
    case "author":
        a.Author = &User{ID: id}
    case "tags":
        a.Tags = append(a.Tags, Tag{ID: id})
    }
    return nil
}
```

### HTTP Server

```go
func main() {
    // Create relationship handlers
    articleRelationships := jsonapi.RelationshipHandlerMux{
        "author": jsonapi.RelationshipHandler{
            Get:    http.HandlerFunc(getArticleAuthor),
            Update: http.HandlerFunc(updateArticleAuthor),
        },
        "tags": jsonapi.RelationshipHandler{
            Get: http.HandlerFunc(getArticleTags),
            Add: http.HandlerFunc(addArticleTag),
            Del: http.HandlerFunc(removeArticleTag),
        },
    }

    // Create resource handlers
    articleHandler := jsonapi.ResourceHandler{
        Retrieve: http.HandlerFunc(getArticle),
        List:     http.HandlerFunc(listArticles),
        Create:   http.HandlerFunc(createArticle),
        Update:   http.HandlerFunc(updateArticle),
        Delete:   http.HandlerFunc(deleteArticle),
        Refs:     articleRelationships,
    }

    // Register handlers
    handlers := map[string]jsonapi.ResourceHandler{
        "articles": articleHandler,
    }

    // Create server (recommended approach)
    mux := jsonapi.DefaultServeMux(handlers)
    http.ListenAndServe(":8080", mux)
}

// Alternative: Use with standard library mux
// resolver := &jsonapi.DefaultRequestResolver{}
// mux := http.NewServeMux()
// mux.Handle("/articles", jsonapi.Handle(resolver, http.HandlerFunc(createArticle)))

func createArticle(w http.ResponseWriter, r *http.Request) {
    req := jsonapi.FromContext(r.Context())

    var article Article
    err := req.Unmarshal(r, &article)
    if err != nil {
        req.WriteErrors(w, http.StatusBadRequest, err)
        return
    }

    // Generate ID and save
    article.ID = generateID()
    articles[article.ID] = article

    // Return with links
    jsonapi.Write(w, http.StatusCreated, article,
        jsonapi.WithDefaultLinks("https://api.example.com"))
}
```

### HTTP Client

```go
func main() {
    // Create a JSON:API client
    client := jsonapi.NewClient("https://api.example.com")

    ctx := context.Background()

    // Fetch a single resource
    resp, err := client.Fetch(ctx, "articles", "1")
    if err != nil {
        // Check if it's a server error response (non-2xx)
        var respErr *jsonapi.ResponseError
        if errors.As(err, &respErr) {
            log.Printf("Server returned %d", respErr.StatusCode)
            for _, e := range respErr.Errors() {
                log.Printf("  %s: %s", e.Title, e.Detail)
            }
        }
        log.Fatal(err)
    }

    var article Article
    resp.Unmarshal(&article)

    // List a collection
    resp, err = client.List(ctx, "articles",
        jsonapi.WithInclude("author", "tags"),
        jsonapi.WithFields("articles", "title", "content"),
        jsonapi.WithSort("-created_at"),
        jsonapi.WithPageNumber(1, 25),
    )
    if err != nil {
        log.Fatal(err)
    }

    var articles []Article
    resp.Unmarshal(&articles)

    // Create a resource
    newArticle := Article{Title: "Hello", Content: "World"}
    resp, err = client.Create(ctx, newArticle)
    if err != nil {
        log.Fatal(err)
    }

    // Update a resource
    article.Title = "Updated Title"
    resp, err = client.Update(ctx, &article)
    if err != nil {
        log.Fatal(err)
    }

    // Delete a resource
    resp, err = client.Delete(ctx, "articles", "1")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Marshaling & Unmarshaling

```go
// Marshal a resource
data, err := jsonapi.Marshal(article,
    jsonapi.WithDefaultLinks("https://api.example.com"),
    jsonapi.WithTopMeta("total", 42))

// Unmarshal from request
var article Article
err := jsonapi.Unmarshal(data, &article)

// Unmarshal relationships
err := jsonapi.UnmarshalRef(data, "author", &article)
```

## Advanced Features

### Custom Link Resolution

```go
type CustomLinkResolver struct {
    BaseURL string
}

func (r CustomLinkResolver) ResolveResourceLink(key string, id jsonapi.ResourceIdentifier) (jsonapi.Link, bool) {
    if key == "self" {
        href := fmt.Sprintf("%s/%s/%s", r.BaseURL, id.ResourceType(), id.ResourceID())
        return jsonapi.Link{Href: href}, true
    }
    return jsonapi.Link{}, false
}

func (r CustomLinkResolver) ResolveRelationshipLink(key string, name string, id jsonapi.RelationshipMarshaler) (jsonapi.Link, bool) {
    // Custom relationship link logic
    return jsonapi.Link{}, false
}

// Use custom resolver
jsonapi.Marshal(article, jsonapi.WithLinkResolver("self", resolver))
```

### Error Handling

```go
// Return JSON:API errors
err := &jsonapi.Error{
    Status: "422",
    Title:  "Validation Error",
    Detail: "Title cannot be empty",
    Source: &jsonapi.ErrorSource{Pointer: "/data/attributes/title"},
}
jsonapi.Marshal(nil, jsonapi.WithError(err))
```

### Include Related Resources

```go
// Marshal with included resources
jsonapi.Marshal(article,
    jsonapi.WithMaxIncludeDepth(2),
    jsonapi.WithDefaultLinks("https://api.example.com"))
```

## HTTP Client

The library includes a typed HTTP client for consuming JSON:API servers. The client reuses the same resource interfaces used on the server side, so the same struct definitions work for both producing and consuming JSON:API documents.

### Creating a Client

```go
client := jsonapi.NewClient("https://api.example.com")
```

Customize the client with options:

```go
client := jsonapi.NewClient("https://api.example.com",
    jsonapi.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
    jsonapi.WithURLResolver(myResolver),
    jsonapi.WithRequestMiddleware(authMiddleware),
    jsonapi.WithResponseMiddleware(loggingMiddleware),
)
```

### CRUD Operations

```go
ctx := context.Background()

// Fetch a single resource
resp, err := client.Fetch(ctx, "articles", "1")
var article Article
resp.Unmarshal(&article)

// List a collection
resp, err := client.List(ctx, "articles")
var articles []Article
resp.Unmarshal(&articles)

// Create a resource
resp, err := client.Create(ctx, newArticle)

// Update a resource
resp, err := client.Update(ctx, &article)

// Delete a resource
resp, err := client.Delete(ctx, "articles", "1")
```

### Relationship Operations

```go
// Fetch relationship identifiers
resp, err := client.FetchRef(ctx, "articles", "1", "author")
var article Article
resp.UnmarshalRef("author", &article)

// Replace a relationship
resp, err := client.UpdateRef(ctx, &article, "author")

// Add to a to-many relationship
resp, err := client.AddRef(ctx, &article, "tags")

// Remove from a to-many relationship
resp, err := client.RemoveRef(ctx, &article, "tags")

// Fetch full related resources
resp, err := client.FetchRelated(ctx, "articles", "1", "comments")
var comments []Comment
resp.Unmarshal(&comments)
```

### Query Parameters

Compose query parameters as per-request options:

```go
resp, err := client.List(ctx, "articles",
    jsonapi.WithInclude("author", "tags"),
    jsonapi.WithFields("articles", "title", "content"),
    jsonapi.WithFields("users", "name"),
    jsonapi.WithSort("-created_at", "title"),
    jsonapi.WithFilter("status", "published"),
)
```

Pagination options:

```go
// Number-based pagination: ?page[number]=1&page[size]=25
jsonapi.WithPageNumber(1, 25)

// Cursor-based pagination: ?page[after]=abc123&page[size]=25
jsonapi.WithPageCursor("abc123", 25)

// Custom page parameters: ?page[offset]=10&page[limit]=25
jsonapi.WithPageParams(map[string]string{"offset": "10", "limit": "25"})
```

Raw query parameters for non-standard or vendor-specific needs:

```go
resp, err := client.List(ctx, "articles",
    jsonapi.WithQueryParam("search", "golang"),
    jsonapi.WithQueryParam("version", "2"),
)
// produces: ?search=golang&version=2
```

### Page Iterator

Traverse paginated collections by following `next` links automatically:

```go
resp, err := client.List(ctx, "articles", jsonapi.WithPageNumber(1, 25))
if err != nil {
    log.Fatal(err)
}

// Process the first page
var articles []Article
resp.Unmarshal(&articles)
process(articles)

// Iterate through remaining pages
iter := client.Pages(resp)
for iter.Next(ctx) {
    iter.Items(&articles)
    process(articles)
}
if err := iter.Err(); err != nil {
    log.Fatal(err)
}
```

### Unmarshal Included Resources

Extract included resources by type from a response:

```go
resp, err := client.Fetch(ctx, "articles", "1",
    jsonapi.WithInclude("author", "tags"),
)

var article Article
resp.Unmarshal(&article)

var author []User
resp.UnmarshalIncluded("users", &author)

var tags []Tag
resp.UnmarshalIncluded("tags", &tags)
```

### Response Access

The `Response` type provides access to the full JSON:API document:

```go
resp.StatusCode          // HTTP status code
resp.Header              // HTTP response headers
resp.Document()          // Full *Document
resp.Links()             // Top-level links
resp.Meta()              // Top-level meta
resp.Errors()            // JSON:API error objects
resp.HasErrors()         // true if errors are present
```

### Error Handling

All non-2xx HTTP responses return a `*ResponseError` as the error value (with a `nil` response). Use `errors.As` to access the response details:

```go
resp, err := client.Fetch(ctx, "articles", "999")
if err != nil {
    var respErr *jsonapi.ResponseError
    if errors.As(err, &respErr) {
        fmt.Println(respErr.StatusCode) // e.g. 404
        for _, e := range respErr.Errors() {
            fmt.Println(e.Title, e.Detail)
        }
    }
    return err
}

// resp is guaranteed non-nil here (2xx response)
var article Article
resp.Unmarshal(&article)
```

`ResponseError` embeds `*Response`, so all the same accessors (`StatusCode`, `Header`, `HasErrors()`, `Errors()`, `Document()`) are available on the error value. Network errors, context cancellation, and marshal failures return plain errors that are not `*ResponseError`.

### Custom URL Resolver

Implement `URLResolver` to work with servers that use non-standard URL patterns:

```go
type CustomResolver struct {
    BaseURL string
}

func (r CustomResolver) ResolveResourceURL(resourceType, id string) string {
    return r.BaseURL + "/api/v2/" + resourceType + "/" + id
}

func (r CustomResolver) ResolveCollectionURL(resourceType string) string {
    return r.BaseURL + "/api/v2/" + resourceType
}

func (r CustomResolver) ResolveRelationshipURL(resourceType, id, relationship string) string {
    return r.BaseURL + "/api/v2/" + resourceType + "/" + id + "/links/" + relationship
}

func (r CustomResolver) ResolveRelatedURL(resourceType, id, relationship string) string {
    return r.BaseURL + "/api/v2/" + resourceType + "/" + id + "/" + relationship
}

client := jsonapi.NewClient("", jsonapi.WithURLResolver(CustomResolver{
    BaseURL: "https://api.example.com",
}))
```

### Request and Response Middleware

Add cross-cutting concerns like authentication and logging:

```go
// Request middleware: add an auth header to every request
authMiddleware := func(r *http.Request) (*http.Request, error) {
    r.Header.Set("Authorization", "Bearer "+token)
    return r, nil
}

// Response middleware: log response status
logMiddleware := func(r *http.Response) (*http.Response, error) {
    log.Printf("%s %s → %d", r.Request.Method, r.Request.URL, r.StatusCode)
    return r, nil
}

client := jsonapi.NewClient("https://api.example.com",
    jsonapi.WithRequestMiddleware(authMiddleware),
    jsonapi.WithResponseMiddleware(logMiddleware),
)
```

## API Reference

### Core Types

- `ResourceIdentifier` - Basic resource identification
- `RelationshipMarshaler` - Resource with relationships
- `LinksMarshaler` - Resource with custom links
- `MetaMarshaler` - Resource with metadata
- `RelationshipUnmarshaler` - Resource that can receive relationship updates

### Options

- `WithDefaultLinks(baseURL)` - Add standard JSON:API links
- `WithLinkResolver(key, resolver)` - Custom link generation
- `WithTopMeta(key, value)` - Top-level metadata
- `WithMaxIncludeDepth(depth)` - Limit relationship inclusion
- `WithTypeValidation()` - Enable type validation
- `WithError(status, err)` - Add errors to response
- `WithInclude(relationships...)` - Include related resources in client requests
- `WithFields(resourceType, fields...)` - Sparse fieldsets for client requests
- `WithSort(fields...)` - Sort fields for client requests
- `WithPageNumber(number, size)` - Number-based pagination
- `WithPageCursor(cursor, size)` - Cursor-based pagination
- `WithPageParams(params)` - Custom page parameters
- `WithFilter(key, value)` - Filter parameters
- `WithQueryParam(key, value)` - Raw query parameters

### HTTP Server Utilities

- `DefaultServeMux(handlers)` - Create JSON:API HTTP multiplexer with resource handlers
- `FromContext(ctx)` - Extract request info from context
- `Write(w, status, resource, opts...)` - Write JSON:API response
- `WriteErrors(w, status, errors...)` - Write error response

### HTTP Client

- `NewClient(baseURL, opts...)` - Create a JSON:API HTTP client
- `WithHTTPClient(hc)` - Set a custom `net/http.Client`
- `WithURLResolver(r)` - Set a custom URL resolver
- `WithRequestMiddleware(m...)` - Add request middleware
- `WithResponseMiddleware(m...)` - Add response middleware
- `Client.Fetch(ctx, type, id, opts...)` - GET a single resource
- `Client.List(ctx, type, opts...)` - GET a resource collection
- `Client.Create(ctx, resource, opts...)` - POST a new resource
- `Client.Update(ctx, resource, opts...)` - PATCH an existing resource
- `Client.Delete(ctx, type, id, opts...)` - DELETE a resource
- `Client.FetchRef(ctx, type, id, ref, opts...)` - GET relationship data
- `Client.UpdateRef(ctx, resource, ref, opts...)` - PATCH relationship data
- `Client.AddRef(ctx, resource, ref, opts...)` - POST to a to-many relationship
- `Client.RemoveRef(ctx, resource, ref, opts...)` - DELETE from a to-many relationship
- `Client.FetchRelated(ctx, type, id, ref, opts...)` - GET related resources
- `Client.Pages(response)` - Create a page iterator
- `Response.Unmarshal(target, opts...)` - Unmarshal primary data
- `Response.UnmarshalRef(name, target, opts...)` - Unmarshal relationship data
- `Response.UnmarshalIncluded(type, target, opts...)` - Unmarshal included resources by type
- `ResponseError` - Error type for non-2xx responses, embeds `*Response`; use `errors.As` to extract from `error`

## Migration from v1

v2 introduces several improvements:

- **Link Resolvers**: Replace server-aware resources with pluggable URL generation
- **Request Methods**: `req.Unmarshal()`, `req.UnmarshalRef()` eliminate boilerplate
- **Better Type Safety**: Improved interface constraints and validation
- **Enhanced Documentation**: Comprehensive godoc with cross-references

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## JSON:API Specification

This library implements [JSON:API v1.1](https://jsonapi.org/format/1.1/). For detailed specification information, visit the official JSON:API website.
