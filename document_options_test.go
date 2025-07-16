package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentMeta(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define metadata
	meta := map[string]interface{}{
		"count":    42,
		"page":     1,
		"per_page": 10,
		"total":    100,
	}

	// Marshal with DocumentMeta option
	data, err := Marshal(resource, DocumentMeta(meta))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify metadata was included (note: JSON unmarshaling converts numbers to float64)
	assert.Equal(t, float64(42), doc.Meta["count"])
	assert.Equal(t, float64(1), doc.Meta["page"])
	assert.Equal(t, float64(10), doc.Meta["per_page"])
	assert.Equal(t, float64(100), doc.Meta["total"])
	
	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}

func TestDocumentLinks(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define links
	links := map[string]Link{
		"self":  {Href: "/api/test/1"},
		"first": {Href: "/api/test?page=1"},
		"last":  {Href: "/api/test?page=10"},
		"next":  {Href: "/api/test?page=2"},
		"prev":  {Href: "", Meta: map[string]interface{}{"available": false}},
	}

	// Marshal with DocumentLinks option
	data, err := Marshal(resource, DocumentLinks(links))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify links were included
	assert.Equal(t, "/api/test/1", doc.Links["self"].Href)
	assert.Equal(t, "/api/test?page=1", doc.Links["first"].Href)
	assert.Equal(t, "/api/test?page=10", doc.Links["last"].Href)
	assert.Equal(t, "/api/test?page=2", doc.Links["next"].Href)
	
	// The empty href with meta will be marshaled as null due to the MarshalJSON implementation
	assert.Equal(t, "", doc.Links["prev"].Href)
	assert.Nil(t, doc.Links["prev"].Meta)
	
	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}

func TestDocumentMetaAndLinks(t *testing.T) {
	// Create a simple resource
	resource := struct {
		ID   string `jsonapi:"primary,test"`
		Name string `jsonapi:"attr,name"`
	}{
		ID:   "1",
		Name: "Test Resource",
	}

	// Define metadata and links
	meta := map[string]interface{}{
		"count": 42,
		"total": 100,
	}
	
	links := map[string]Link{
		"self": {Href: "/api/test/1"},
		"next": {Href: "/api/test?page=2"},
	}

	// Marshal with both DocumentMeta and DocumentLinks options
	data, err := Marshal(resource, DocumentMeta(meta), DocumentLinks(links))
	require.NoError(t, err)

	// Unmarshal to verify
	var doc Document
	err = json.Unmarshal(data, &doc)
	require.NoError(t, err)

	// Verify metadata was included (note: JSON unmarshaling converts numbers to float64)
	assert.Equal(t, float64(42), doc.Meta["count"])
	assert.Equal(t, float64(100), doc.Meta["total"])
	
	// Verify links were included
	assert.Equal(t, "/api/test/1", doc.Links["self"].Href)
	assert.Equal(t, "/api/test?page=2", doc.Links["next"].Href)
	
	// Verify resource data is still present
	resourceData, ok := doc.Data.One()
	require.True(t, ok)
	assert.Equal(t, "1", resourceData.ID)
	assert.Equal(t, "test", resourceData.Type)
	assert.Equal(t, "Test Resource", resourceData.Attributes["name"])
}
