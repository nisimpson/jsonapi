package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplySparseFieldsets(t *testing.T) {
	// Create a resource with attributes
	resource := Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		},
	}
	
	// Apply sparse fieldsets
	resource.ApplySparseFieldsets([]string{"name", "email"})
	
	// Verify only the specified fields are included
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])
	assert.NotContains(t, resource.Attributes, "age")
	
	// Verify the resource has only the specified attributes
	assert.Len(t, resource.Attributes, 2)
	
	// Test with empty fields (should keep all attributes)
	resource = Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
	}
	resource.ApplySparseFieldsets([]string{})
	assert.Len(t, resource.Attributes, 2)
	
	// Test with nil attributes
	resource = Resource{
		ID:   "1",
		Type: "users",
		Attributes: nil,
	}
	resource.ApplySparseFieldsets([]string{"name"})
	assert.NotNil(t, resource.Attributes)
	assert.Len(t, resource.Attributes, 0)
}

func TestSparseFieldsetsOption(t *testing.T) {
	// Create MarshalOptions with SparseFieldsets
	options := &MarshalOptions{}
	SparseFieldsets("users", []string{"name"})(options)
	
	// Verify the modifyDocument function was added
	assert.Len(t, options.modifyDocument, 1)
	
	// Create a document to test the modifyDocument function
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
	
	// Apply the modifyDocument function
	options.modifyDocument[0](doc)
	
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
