package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplySparseFieldsetsDirectly(t *testing.T) {
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
}
