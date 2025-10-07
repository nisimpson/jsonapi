# JSON:API Library for Go

[![Test](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml/badge.svg)](https://github.com/nisimpson/jsonapi/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/nisimpson/jsonapi?status.svg)](http://godoc.org/github.com/nisimpson/jsonapi)
[![Release](https://img.shields.io/github/release/nisimpson/jsonapi.svg)](https://github.com/nisimpson/jsonapi/releases)

A comprehensive Go library for building JSON:API compliant REST APIs. This library provides marshaling, unmarshaling, and HTTP handling utilities that follow the [JSON:API specification](https://jsonapi.org/).

## Features

- **Complete JSON:API Support**: Resources, relationships, links, meta, errors, and includes
- **Type-Safe Marshaling**: Struct-based resource definitions with interface-driven customization
- **Flexible Unmarshaling**: Support for creation, updates, and relationship operations
- **HTTP Integration**: Built-in mux with automatic request parsing and context injection
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
    ID       string   `json:"-"`
    Title    string   `json:"title"`
    Content  string   `json:"content"`
    AuthorID string   `json:"author"`
    TagIDs   []string `json:"_"`
}

func (a Article) ResourceType() string { return "articles" }
func (a Article) ResourceID() string { return a.ID }

// Implement relationships
func (a Article) Relationships() map[string]jsonapi.RelationType {
    return map[string]jsonapi.RelationType{
        "author": jsonapi.RelationToOne,
        "tags":   jsonapi.RelationToMany,
    }
}

func (a *Article) SetRelation(name, id string, meta map[string]interface{}) error {
    switch name {
    case "author":
        a.AuthorID = id
    case "tags":
        a.TagIDs = append(a.TagIDs, id)
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

### HTTP Utilities

- `DefaultServeMux(handlers)` - Create JSON:API HTTP multiplexer with resource handlers
- `FromContext(ctx)` - Extract request info from context
- `Write(w, status, resource, opts...)` - Write JSON:API response
- `WriteErrors(w, status, errors...)` - Write error response

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
