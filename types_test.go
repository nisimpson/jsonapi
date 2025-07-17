package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResource tests the Resource type
func TestResource(t *testing.T) {
	// Create a resource
	resource := Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
		Relationships: map[string]Relationship{
			"posts": {
				Data: MultiResource(
					Resource{ID: "101", Type: "posts"},
					Resource{ID: "102", Type: "posts"},
				),
			},
		},
		Links: map[string]Link{
			"self": {Href: "/users/1"},
		},
		Meta: map[string]interface{}{
			"created_at": "2023-01-01T00:00:00Z",
		},
	}

	// Verify the resource properties
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "John Doe", resource.Attributes["name"])
	assert.Equal(t, "john@example.com", resource.Attributes["email"])

	// Verify relationships
	relationship, ok := resource.Relationships["posts"]
	require.True(t, ok)
	resources, ok := relationship.Data.Many()
	require.True(t, ok)
	require.Len(t, resources, 2)
	assert.Equal(t, "101", resources[0].ID)
	assert.Equal(t, "posts", resources[0].Type)
	assert.Equal(t, "102", resources[1].ID)
	assert.Equal(t, "posts", resources[1].Type)

	// Verify links
	assert.Equal(t, "/users/1", resource.Links["self"].Href)

	// Verify meta
	assert.Equal(t, "2023-01-01T00:00:00Z", resource.Meta["created_at"])
}

// TestPrimaryData tests the PrimaryData type
func TestPrimaryData(t *testing.T) {
	t.Run("null resource", func(t *testing.T) {
		data := NullResource()
		assert.True(t, data.Null())
		_, ok := data.One()
		assert.False(t, ok)
		_, ok = data.Many()
		assert.False(t, ok)
	})

	t.Run("single resource", func(t *testing.T) {
		resource := Resource{ID: "1", Type: "users"}
		data := SingleResource(resource)
		assert.False(t, data.Null())

		one, ok := data.One()
		assert.True(t, ok)
		assert.Equal(t, "1", one.ID)
		assert.Equal(t, "users", one.Type)

		_, ok = data.Many()
		assert.False(t, ok)
	})

	t.Run("multiple resources", func(t *testing.T) {
		resources := []Resource{
			{ID: "1", Type: "users"},
			{ID: "2", Type: "users"},
		}
		data := MultiResource(resources...)
		assert.False(t, data.Null())

		_, ok := data.One()
		assert.False(t, ok)

		many, ok := data.Many()
		assert.True(t, ok)
		require.Len(t, many, 2)
		assert.Equal(t, "1", many[0].ID)
		assert.Equal(t, "users", many[0].Type)
		assert.Equal(t, "2", many[1].ID)
		assert.Equal(t, "users", many[1].Type)
	})
}

// TestPrimaryData_Iter tests the Iter method of PrimaryData
func TestPrimaryData_Iter(t *testing.T) {
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
			expected: nil,
		},
		{
			name:     "empty resource array",
			data:     MultiResource(),
			expected: nil,
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

// TestPrimaryData_Iter_EarlyTermination tests early termination of the Iter method
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

// TestPrimaryData_MarshalJSON tests marshaling PrimaryData to JSON
func TestPrimaryData_MarshalJSON(t *testing.T) {
	t.Run("null resource", func(t *testing.T) {
		data := NullResource()
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)
		assert.Equal(t, "null", string(jsonData))
	})

	t.Run("single resource", func(t *testing.T) {
		resource := Resource{ID: "1", Type: "users"}
		data := SingleResource(resource)
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		assert.Equal(t, "1", result["id"])
		assert.Equal(t, "users", result["type"])
	})

	t.Run("multiple resources", func(t *testing.T) {
		resources := []Resource{
			{ID: "1", Type: "users"},
			{ID: "2", Type: "users"},
		}
		data := MultiResource(resources...)
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)

		var result []map[string]interface{}
		err = json.Unmarshal(jsonData, &result)
		require.NoError(t, err)

		require.Len(t, result, 2)
		assert.Equal(t, "1", result[0]["id"])
		assert.Equal(t, "users", result[0]["type"])
		assert.Equal(t, "2", result[1]["id"])
		assert.Equal(t, "users", result[1]["type"])
	})
}

// TestPrimaryData_UnmarshalJSON tests unmarshaling JSON to PrimaryData
func TestPrimaryData_UnmarshalJSON(t *testing.T) {
	t.Run("null resource", func(t *testing.T) {
		jsonData := []byte("null")
		var data PrimaryData
		err := json.Unmarshal(jsonData, &data)
		require.NoError(t, err)
		assert.True(t, data.Null())
	})

	t.Run("single resource", func(t *testing.T) {
		jsonData := []byte(`{"id":"1","type":"users"}`)
		var data PrimaryData
		err := json.Unmarshal(jsonData, &data)
		require.NoError(t, err)

		resource, ok := data.One()
		require.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
	})

	t.Run("multiple resources", func(t *testing.T) {
		jsonData := []byte(`[{"id":"1","type":"users"},{"id":"2","type":"users"}]`)
		var data PrimaryData
		err := json.Unmarshal(jsonData, &data)
		require.NoError(t, err)

		resources, ok := data.Many()
		require.True(t, ok)
		require.Len(t, resources, 2)
		assert.Equal(t, "1", resources[0].ID)
		assert.Equal(t, "users", resources[0].Type)
		assert.Equal(t, "2", resources[1].ID)
		assert.Equal(t, "users", resources[1].Type)
	})
}

// TestLink tests the Link type
func TestLink_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		link     Link
		expected string
	}{
		{
			name: "simple href",
			link: Link{
				Href: "/api/test/1",
			},
			expected: `"/api/test/1"`,
		},
		{
			name: "href with meta",
			link: Link{
				Href: "/api/test/1",
				Meta: map[string]interface{}{
					"type": "primary",
					"info": "additional information",
				},
			},
			expected: `{"href":"/api/test/1","meta":{"info":"additional information","type":"primary"}}`,
		},
		{
			name:     "empty link",
			link:     Link{},
			expected: `null`,
		},
		{
			name: "empty href with meta",
			link: Link{
				Meta: map[string]interface{}{
					"type": "primary",
				},
			},
			expected: `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.link)
			require.NoError(t, err)

			// For object comparison, we need to normalize the JSON
			if tt.expected[0] == '{' {
				var expected, actual interface{}
				err = json.Unmarshal([]byte(tt.expected), &expected)
				require.NoError(t, err)
				err = json.Unmarshal(data, &actual)
				require.NoError(t, err)
				assert.Equal(t, expected, actual)
			} else {
				assert.Equal(t, tt.expected, string(data))
			}
		})
	}
}

// TestLink_UnmarshalJSON tests unmarshaling JSON to Link
func TestLink_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected Link
	}{
		{
			name: "string href",
			json: `"/api/test/1"`,
			expected: Link{
				Href: "/api/test/1",
			},
		},
		{
			name: "object with href",
			json: `{"href":"/api/test/1"}`,
			expected: Link{
				Href: "/api/test/1",
			},
		},
		{
			name: "object with href and meta",
			json: `{"href":"/api/test/1","meta":{"type":"primary"}}`,
			expected: Link{
				Href: "/api/test/1",
				Meta: map[string]interface{}{
					"type": "primary",
				},
			},
		},
		{
			name:     "null",
			json:     `null`,
			expected: Link{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var link Link
			err := json.Unmarshal([]byte(tt.json), &link)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Href, link.Href)

			if tt.expected.Meta == nil {
				assert.Nil(t, link.Meta)
			} else {
				assert.Equal(t, tt.expected.Meta, link.Meta)
			}
		})
	}
}

// TestError tests the Error type
func TestError(t *testing.T) {
	// Create an error
	err := Error{
		ID:     "123",
		Status: "404",
		Code:   "not_found",
		Title:  "Resource Not Found",
		Detail: "The requested resource could not be found",
		Source: map[string]interface{}{
			"pointer":   "/data/attributes/title",
			"parameter": "title",
		},
		Links: map[string]interface{}{
			"about": "/errors/not_found",
		},
	}

	// Verify the error properties
	assert.Equal(t, "123", err.ID)
	assert.Equal(t, "404", err.Status)
	assert.Equal(t, "not_found", err.Code)
	assert.Equal(t, "Resource Not Found", err.Title)
	assert.Equal(t, "The requested resource could not be found", err.Detail)
	assert.Equal(t, "/data/attributes/title", err.Source["pointer"])
	assert.Equal(t, "title", err.Source["parameter"])
	assert.Equal(t, "/errors/not_found", err.Links["about"])

	// Test the Error method
	assert.Equal(t, "Resource Not Found: The requested resource could not be found (not_found)", err.Error())
}

// TestMultiError tests the MultiError type
func TestMultiError(t *testing.T) {
	// Create a multi-error
	multiErr := MultiError{
		{
			Status: "400",
			Title:  "Bad Request",
			Detail: "Invalid request format",
		},
		{
			Status: "422",
			Title:  "Validation Error",
			Detail: "Field 'name' is required",
		},
	}

	// Verify the multi-error properties
	require.Len(t, multiErr, 2)
	assert.Equal(t, "400", multiErr[0].Status)
	assert.Equal(t, "Bad Request", multiErr[0].Title)
	assert.Equal(t, "Invalid request format", multiErr[0].Detail)
	assert.Equal(t, "422", multiErr[1].Status)
	assert.Equal(t, "Validation Error", multiErr[1].Title)
	assert.Equal(t, "Field 'name' is required", multiErr[1].Detail)

	// Test the Error method
	assert.Contains(t, multiErr.Error(), "Bad Request: Invalid request format")
	assert.Contains(t, multiErr.Error(), "Validation Error: Field 'name' is required")
}
