package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecCompliance_ResourceObject tests compliance with the JSON:API spec for resource objects
func TestSpecCompliance_ResourceObject(t *testing.T) {
	// Create a resource object
	resource := Resource{
		ID:   "1",
		Type: "articles",
		Attributes: map[string]interface{}{
			"title": "JSON:API paints my bikeshed!",
		},
		Relationships: map[string]Relationship{
			"author": {
				Links: map[string]Link{
					"self":    {Href: "/articles/1/relationships/author"},
					"related": {Href: "/articles/1/author"},
				},
				Data: SingleResource(Resource{
					ID:   "9",
					Type: "people",
				}),
			},
			"comments": {
				Links: map[string]Link{
					"self":    {Href: "/articles/1/relationships/comments"},
					"related": {Href: "/articles/1/comments"},
				},
				Data: MultiResource(
					Resource{ID: "5", Type: "comments"},
					Resource{ID: "12", Type: "comments"},
				),
			},
		},
		Links: map[string]Link{
			"self": {Href: "/articles/1"},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(resource)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify resource object structure according to spec
	assert.Equal(t, "1", result["id"])
	assert.Equal(t, "articles", result["type"])

	// Verify attributes
	attributes, ok := result["attributes"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "JSON:API paints my bikeshed!", attributes["title"])

	// Verify relationships
	relationships, ok := result["relationships"].(map[string]interface{})
	require.True(t, ok)

	// Verify author relationship
	author, ok := relationships["author"].(map[string]interface{})
	require.True(t, ok)
	authorLinks, ok := author["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles/1/relationships/author", authorLinks["self"])
	assert.Equal(t, "/articles/1/author", authorLinks["related"])
	authorData, ok := author["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "9", authorData["id"])
	assert.Equal(t, "people", authorData["type"])

	// Verify comments relationship
	comments, ok := relationships["comments"].(map[string]interface{})
	require.True(t, ok)
	commentsLinks, ok := comments["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles/1/relationships/comments", commentsLinks["self"])
	assert.Equal(t, "/articles/1/comments", commentsLinks["related"])
	commentsData, ok := comments["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, commentsData, 2)
	comment1, ok := commentsData[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "5", comment1["id"])
	assert.Equal(t, "comments", comment1["type"])
	comment2, ok := commentsData[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "12", comment2["id"])
	assert.Equal(t, "comments", comment2["type"])

	// Verify links
	links, ok := result["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles/1", links["self"])
}

// TestSpecCompliance_Document tests compliance with the JSON:API spec for document structure
func TestSpecCompliance_Document(t *testing.T) {
	// Create a document
	doc := Document{
		Data: SingleResource(Resource{
			ID:   "1",
			Type: "articles",
			Attributes: map[string]interface{}{
				"title": "JSON:API paints my bikeshed!",
			},
			Relationships: map[string]Relationship{
				"author": {
					Data: SingleResource(Resource{
						ID:   "9",
						Type: "people",
					}),
				},
			},
		}),
		Included: []Resource{
			{
				ID:   "9",
				Type: "people",
				Attributes: map[string]interface{}{
					"name": "John Doe",
				},
			},
		},
		Meta: map[string]interface{}{
			"copyright": "Copyright 2023 Example Corp.",
		},
		Links: map[string]Link{
			"self": {Href: "/articles/1"},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(doc)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify document structure according to spec
	primaryData, ok := result["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1", primaryData["id"])
	assert.Equal(t, "articles", primaryData["type"])

	// Verify included resources
	included, ok := result["included"].([]interface{})
	require.True(t, ok)
	require.Len(t, included, 1)
	includedResource, ok := included[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "9", includedResource["id"])
	assert.Equal(t, "people", includedResource["type"])

	// Verify meta
	meta, ok := result["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Copyright 2023 Example Corp.", meta["copyright"])

	// Verify links
	links, ok := result["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles/1", links["self"])
}

// TestSpecCompliance_Errors tests compliance with the JSON:API spec for error objects
func TestSpecCompliance_Errors(t *testing.T) {
	// Create an error document
	errorDoc := Document{
		Errors: []Error{
			{
				ID:     "123",
				Status: "422",
				Code:   "validation_error",
				Title:  "Invalid Attribute",
				Detail: "First name must contain at least three characters.",
				Source: map[string]interface{}{
					"pointer": "/data/attributes/firstName",
				},
				Links: map[string]interface{}{
					"about": "/errors/validation",
				},
			},
			{
				ID:     "124",
				Status: "422",
				Code:   "validation_error",
				Title:  "Invalid Attribute",
				Detail: "Password must contain a special character.",
				Source: map[string]interface{}{
					"pointer": "/data/attributes/password",
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(errorDoc)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify error structure according to spec
	errors, ok := result["errors"].([]interface{})
	require.True(t, ok)
	require.Len(t, errors, 2)

	// Verify first error
	error1, ok := errors[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "123", error1["id"])
	assert.Equal(t, "422", error1["status"])
	assert.Equal(t, "validation_error", error1["code"])
	assert.Equal(t, "Invalid Attribute", error1["title"])
	assert.Equal(t, "First name must contain at least three characters.", error1["detail"])
	source1, ok := error1["source"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/data/attributes/firstName", source1["pointer"])
	links1, ok := error1["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/errors/validation", links1["about"])

	// Verify second error
	error2, ok := errors[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "124", error2["id"])
	assert.Equal(t, "422", error2["status"])
	assert.Equal(t, "validation_error", error2["code"])
	assert.Equal(t, "Invalid Attribute", error2["title"])
	assert.Equal(t, "Password must contain a special character.", error2["detail"])
	source2, ok := error2["source"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/data/attributes/password", source2["pointer"])
}

// TestSpecCompliance_Relationships tests compliance with the JSON:API spec for relationship objects
func TestSpecCompliance_Relationships(t *testing.T) {
	// Create a relationship
	relationship := Relationship{
		Links: map[string]Link{
			"self":    {Href: "/articles/1/relationships/author"},
			"related": {Href: "/articles/1/author"},
		},
		Data: SingleResource(Resource{
			ID:   "9",
			Type: "people",
		}),
		Meta: map[string]interface{}{
			"count": 1,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(relationship)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify relationship structure according to spec
	links, ok := result["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles/1/relationships/author", links["self"])
	assert.Equal(t, "/articles/1/author", links["related"])

	resourceData, ok := result["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "9", resourceData["id"])
	assert.Equal(t, "people", resourceData["type"])

	meta, ok := result["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1), meta["count"])
}

// TestSpecCompliance_Links tests compliance with the JSON:API spec for link objects
func TestSpecCompliance_Links(t *testing.T) {
	t.Run("string link", func(t *testing.T) {
		// Create a simple link
		link := Link{
			Href: "/articles/1",
		}

		// Marshal to JSON
		data, err := json.Marshal(link)
		require.NoError(t, err)

		// Verify link is marshaled as a string
		assert.Equal(t, `"/articles/1"`, string(data))
	})

	t.Run("object link", func(t *testing.T) {
		// Create a link with meta
		link := Link{
			Href: "/articles/1",
			Meta: map[string]interface{}{
				"count": 10,
			},
		}

		// Marshal to JSON
		data, err := json.Marshal(link)
		require.NoError(t, err)

		// Unmarshal to verify structure
		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		// Verify link structure according to spec
		assert.Equal(t, "/articles/1", result["href"])
		meta, ok := result["meta"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(10), meta["count"])
	})
}

// TestSpecCompliance_Pagination tests compliance with the JSON:API spec for pagination
func TestSpecCompliance_Pagination(t *testing.T) {
	// Create a document with pagination links
	doc := Document{
		Data: MultiResource(
			Resource{ID: "1", Type: "articles"},
			Resource{ID: "2", Type: "articles"},
		),
		Links: map[string]Link{
			"self":  {Href: "/articles?page[number]=3&page[size]=10"},
			"first": {Href: "/articles?page[number]=1&page[size]=10"},
			"prev":  {Href: "/articles?page[number]=2&page[size]=10"},
			"next":  {Href: "/articles?page[number]=4&page[size]=10"},
			"last":  {Href: "/articles?page[number]=5&page[size]=10"},
		},
		Meta: map[string]interface{}{
			"totalPages": 5,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(doc)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify pagination links according to spec
	links, ok := result["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/articles?page[number]=3&page[size]=10", links["self"])
	assert.Equal(t, "/articles?page[number]=1&page[size]=10", links["first"])
	assert.Equal(t, "/articles?page[number]=2&page[size]=10", links["prev"])
	assert.Equal(t, "/articles?page[number]=4&page[size]=10", links["next"])
	assert.Equal(t, "/articles?page[number]=5&page[size]=10", links["last"])

	// Verify meta
	meta, ok := result["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(5), meta["totalPages"])
}

// TestSpecCompliance_SparseFieldsets tests compliance with the JSON:API spec for sparse fieldsets
func TestSpecCompliance_SparseFieldsets(t *testing.T) {
	// Define a test struct
	type User struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email"`
		Age   int    `jsonapi:"attr,age"`
	}

	user := User{
		ID:    "1",
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Marshal with SparseFieldsets option
	data, err := Marshal(user, SparseFieldsets("users", []string{"name"}))
	assert.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)

	// Verify only the name attribute is included
	resource, ok := doc.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.NotContains(t, resource.Attributes, "email")
	assert.NotContains(t, resource.Attributes, "age")
}
