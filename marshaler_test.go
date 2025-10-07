package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testResource struct {
	ID   string `jsonapi:"primary,test"`
	Name string `jsonapi:"attr,name"`
}

func (t testResource) ResourceID() string   { return t.ID }
func (t testResource) ResourceType() string { return "test" }
func (t *testResource) SetResourceID(id string) error {
	t.ID = id
	return nil
}

func TestOneRef(t *testing.T) {
	tests := []struct {
		name     string
		input    ResourceIdentifier
		expected []ResourceIdentifier
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty resource ID returns nil",
			input:    testResource{ID: "", Name: "test"},
			expected: nil,
		},
		{
			name:  "valid resource returns slice with one element",
			input: testResource{ID: "1", Name: "test"},
			expected: []ResourceIdentifier{
				testResource{ID: "1", Name: "test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OneRef(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManyRef(t *testing.T) {
	tests := []struct {
		name     string
		input    []ResourceIdentifier
		expected []ResourceIdentifier
	}{
		{
			name:     "empty input returns empty slice",
			input:    []ResourceIdentifier{},
			expected: []ResourceIdentifier{},
		},
		{
			name: "single resource",
			input: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
			},
			expected: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
			},
		},
		{
			name: "multiple resources",
			input: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
				testResource{ID: "2", Name: "test2"},
			},
			expected: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
				testResource{ID: "2", Name: "test2"},
			},
		},
		{
			name: "includes all resources even with empty IDs",
			input: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
				testResource{ID: "", Name: "empty"},
				testResource{ID: "2", Name: "test2"},
			},
			expected: []ResourceIdentifier{
				testResource{ID: "1", Name: "test1"},
				testResource{ID: "", Name: "empty"},
				testResource{ID: "2", Name: "test2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ManyRef(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarshalMany(t *testing.T) {
	resources := []testResource{
		{ID: "1", Name: "test1"},
		{ID: "2", Name: "test2"},
	}

	data, err := Marshal(resources)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// Verify it's a valid JSON:API document with multiple resources
	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.NotNil(t, doc.Data)
}
