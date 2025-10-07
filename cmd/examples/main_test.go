package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/nisimpson/jsonapi/v2"
)

func Example_getArticle() {
	// Create test server
	handlers := map[string]jsonapi.ResourceHandler{
		"articles": {
			Retrieve: http.HandlerFunc(getArticle),
		},
	}
	mux := jsonapi.DefaultServeMux(handlers)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL + "/articles/1")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// Simplified output check for readability
	if strings.Contains(string(body), `"type":"articles"`) && strings.Contains(string(body), `"id":"1"`) {
		fmt.Println("Article retrieved successfully")
	}

	// Output:
	// Article retrieved successfully
}

func Example_listArticles() {
	// Create test server
	handlers := map[string]jsonapi.ResourceHandler{
		"articles": {
			List: http.HandlerFunc(listArticles),
		},
	}
	mux := jsonapi.DefaultServeMux(handlers)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL + "/articles")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// Simplified output for readability
	if strings.Contains(string(body), `"data":[`) && strings.Contains(string(body), `"type":"articles"`) {
		fmt.Println("JSON:API array response with articles")
	}

	// Output:
	// JSON:API array response with articles
}

func Example_createArticle() {
	// Create test server
	handlers := map[string]jsonapi.ResourceHandler{
		"articles": {
			Create: http.HandlerFunc(createArticle),
		},
	}
	mux := jsonapi.DefaultServeMux(handlers)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Prepare request body
	requestBody := `{
		"data": {
			"type": "articles",
			"attributes": {
				"title": "New Article",
				"content": "This is a new article"
			}
		}
	}`

	// Make request
	req, _ := http.NewRequest("POST", server.URL+"/articles", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/vnd.api+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 201 && strings.Contains(string(body), `"title":"New Article"`) {
		fmt.Println("Article created successfully")
	}

	// Output:
	// Status: 201
	// Article created successfully
}

func Example_getRelationship() {
	// Create test server with relationship handlers
	articleRelationships := jsonapi.RelationshipHandlerMux{
		"author": jsonapi.RelationshipHandler{
			Get: http.HandlerFunc(getArticleAuthor),
		},
		"tags": jsonapi.RelationshipHandler{
			Get: http.HandlerFunc(getArticleTags),
		},
	}

	handlers := map[string]jsonapi.ResourceHandler{
		"articles": {
			Refs: articleRelationships,
		},
	}
	mux := jsonapi.DefaultServeMux(handlers)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Get article author
	resp, err := http.Get(server.URL + "/articles/1/author")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `"type":"users"`) {
		fmt.Println("Author relationship retrieved")
	}

	// Get article tags
	resp2, err := http.Get(server.URL + "/articles/1/tags")
	if err != nil {
		panic(err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	if strings.Contains(string(body2), `"type":"tags"`) {
		fmt.Println("Tags relationship retrieved")
	}

	// Output:
	// Author relationship retrieved
	// Tags relationship retrieved
}

func Example_updateRelationship() {
	// Create test server with relationship handlers
	articleRelationships := jsonapi.RelationshipHandlerMux{
		"author": jsonapi.RelationshipHandler{
			Update: http.HandlerFunc(updateArticleAuthor),
		},
		"tags": jsonapi.RelationshipHandler{
			Add: http.HandlerFunc(addArticleTag),
		},
	}

	handlers := map[string]jsonapi.ResourceHandler{
		"articles": {
			Refs: articleRelationships,
		},
	}
	mux := jsonapi.DefaultServeMux(handlers)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Update author relationship
	authorUpdate := `{"data": {"id": "2", "type": "users"}}`
	req, _ := http.NewRequest("PATCH", server.URL+"/articles/1/relationships/author", strings.NewReader(authorUpdate))
	req.Header.Set("Content-Type", "application/vnd.api+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	fmt.Printf("Author update status: %d\n", resp.StatusCode)

	// Add tags relationship
	tagsAdd := `{"data": [{"id": "3", "type": "tags"}]}`
	req2, _ := http.NewRequest("POST", server.URL+"/articles/1/relationships/tags", strings.NewReader(tagsAdd))
	req2.Header.Set("Content-Type", "application/vnd.api+json")

	resp2, err := client.Do(req2)
	if err != nil {
		panic(err)
	}
	resp2.Body.Close()

	fmt.Printf("Tags add status: %d\n", resp2.StatusCode)

	// Output:
	// Author update status: 204
	// Tags add status: 204
}

func Example_unmarshalRefToOne() {
	// Example of unmarshaling a to-one relationship
	data := `{
		"data": {
			"id": "123",
			"type": "users"
		}
	}`

	var article Article
	err := jsonapi.UnmarshalRef([]byte(data), "author", &article)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Author ID: %s\n", article.AuthorID)

	// Output:
	// Author ID: 123
}

func Example_unmarshalRefToMany() {
	// Example of unmarshaling a to-many relationship
	data := `{
		"data": [
			{"id": "1", "type": "tags"},
			{"id": "2", "type": "tags"}
		]
	}`

	var article Article
	err := jsonapi.UnmarshalRef([]byte(data), "tags", &article)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Tag IDs: %v\n", article.TagIDs)

	// Output:
	// Tag IDs: [1 2]
}

func Example_unmarshalRefNullToOne() {
	// Example of clearing a to-one relationship with null
	data := `{"data": null}`

	var article Article
	article.AuthorID = "existing"

	err := jsonapi.UnmarshalRef([]byte(data), "author", &article)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Author ID after null: '%s'\n", article.AuthorID)

	// Output:
	// Author ID after null: ''
}

func Example_unmarshalRefNullToManyError() {
	// Example showing that null is not allowed for to-many relationships
	data := `{"data": null}`

	var article Article
	err := jsonapi.UnmarshalRef([]byte(data), "tags", &article)

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}

	// Output:
	// Error: null data not allowed for to-many relationship "tags": use empty array instead
}

func Example_marshalBasicResource() {
	// Example of marshaling a basic resource
	article := Article{
		ID:      "1",
		Title:   "Hello JSON:API",
		Content: "This is an example article",
	}

	data, err := jsonapi.Marshal(article)
	if err != nil {
		panic(err)
	}

	// Check key components
	output := string(data)
	if strings.Contains(output, `"id":"1"`) &&
		strings.Contains(output, `"type":"articles"`) &&
		strings.Contains(output, `"title":"Hello JSON:API"`) {
		fmt.Println("Resource marshaled successfully")
	}

	// Output:
	// Resource marshaled successfully
}

func Example_marshalWithRelationships() {
	// Example of marshaling a resource with relationships
	article := Article{
		ID:       "1",
		Title:    "Article with Relationships",
		Content:  "This article has relationships",
		AuthorID: "123",
		TagIDs:   []string{"tag1", "tag2"},
	}

	data, err := jsonapi.Marshal(article)
	if err != nil {
		panic(err)
	}

	// Check for relationships
	output := string(data)
	if strings.Contains(output, `"relationships"`) &&
		strings.Contains(output, `"author"`) &&
		strings.Contains(output, `"tags"`) {
		fmt.Println("Resource with relationships marshaled successfully")
	}

	// Output:
	// Resource with relationships marshaled successfully
}
