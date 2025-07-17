package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSparseFieldsetsSimple(t *testing.T) {
	// Create a document with a resource
	doc := &Document{
		Data: SingleResource(Resource{
			ID:   "1",
			Type: "users",
			Attributes: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		}),
	}
	
	// Create a function that modifies the document
	modifyFunc := func(d *Document) {
		if d.Data.Null() {
			return
		}
		for resource := range d.Data.Iter() {
			if resource.Type == "users" {
				resource.ApplySparseFieldsets([]string{"name"})
			}
		}
	}
	
	// Apply the function
	modifyFunc(doc)
	
	// Get the resource from the document
	resource, ok := doc.Data.One()
	assert.True(t, ok)
	
	// Verify the resource attributes were filtered
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Contains(t, resource.Attributes, "name")
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.NotContains(t, resource.Attributes, "email")
}
