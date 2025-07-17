package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrimaryData_Iter_Additional(t *testing.T) {
	tests := []struct {
		name     string
		data     PrimaryData
		expected []Resource
	}{
		{
			name: "single resource",
			data: SingleResource(Resource{
				ID:   "1",
				Type: "users",
				Attributes: map[string]interface{}{
					"name": "John Doe",
				},
			}),
			expected: []Resource{
				{
					ID:   "1",
					Type: "users",
					Attributes: map[string]interface{}{
						"name": "John Doe",
					},
				},
			},
		},
		{
			name: "multiple resources",
			data: MultiResource(
				Resource{
					ID:   "1",
					Type: "users",
					Attributes: map[string]interface{}{
						"name": "John Doe",
					},
				},
				Resource{
					ID:   "2",
					Type: "users",
					Attributes: map[string]interface{}{
						"name": "Jane Smith",
					},
				},
			),
			expected: []Resource{
				{
					ID:   "1",
					Type: "users",
					Attributes: map[string]interface{}{
						"name": "John Doe",
					},
				},
				{
					ID:   "2",
					Type: "users",
					Attributes: map[string]interface{}{
						"name": "Jane Smith",
					},
				},
			},
		},
		{
			name:     "null resource",
			data:     NullResource(),
			expected: nil, // Changed from empty slice to nil
		},
		{
			name:     "empty resource array",
			data:     MultiResource(),
			expected: nil, // Changed from empty slice to nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Collect resources from iterator
			var resources []Resource
			for resourcePtr := range tt.data.Iter() {
				// Dereference the pointer and make a copy
				resources = append(resources, *resourcePtr)
			}
			
			// Verify the resources match the expected ones
			assert.Equal(t, tt.expected, resources)
		})
	}
}

func TestPrimaryData_Iter_EarlyTermination(t *testing.T) {
	// Create primary data with multiple resources
	data := MultiResource(
		Resource{ID: "1", Type: "users"},
		Resource{ID: "2", Type: "users"},
		Resource{ID: "3", Type: "users"},
		Resource{ID: "4", Type: "users"},
	)
	
	// Test early termination by breaking after the second resource
	var resources []Resource
	count := 0
	for resourcePtr := range data.Iter() {
		// Dereference the pointer and make a copy
		resources = append(resources, *resourcePtr)
		count++
		if count >= 2 {
			break
		}
	}
	
	// Verify only the first two resources were collected
	assert.Len(t, resources, 2)
	assert.Equal(t, "1", resources[0].ID)
	assert.Equal(t, "2", resources[1].ID)
}
