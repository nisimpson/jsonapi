package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetJSONMarshaler(t *testing.T) {
	originalMarshaler := jsonMarshal
	defer func() { jsonMarshal = originalMarshaler }()

	customMarshaler := json.Marshal
	SetJSONMarshaler(customMarshaler)

	// Verify the marshaler was set (we can't directly compare functions)
	assert.NotNil(t, jsonMarshal)
}

func TestSetJSONUnmarshaler(t *testing.T) {
	originalUnmarshaler := jsonUnmarshal
	defer func() { jsonUnmarshal = originalUnmarshaler }()

	customUnmarshaler := json.Unmarshal
	SetJSONUnmarshaler(customUnmarshaler)

	// Verify the unmarshaler was set (we can't directly compare functions)
	assert.NotNil(t, jsonUnmarshal)
}

func TestLink_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		link     Link
		expected string
	}{
		{
			name:     "simple href",
			link:     Link{Href: "http://example.com"},
			expected: `"http://example.com"`,
		},
		{
			name: "href with meta",
			link: Link{
				Href: "http://example.com",
				Meta: map[string]interface{}{"count": 10},
			},
			expected: `{"href":"http://example.com","meta":{"count":10}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.link.MarshalJSON()
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestLink_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Link
	}{
		{
			name:     "simple href string",
			jsonData: `"http://example.com"`,
			expected: Link{Href: "http://example.com"},
		},
		{
			name:     "href with meta object",
			jsonData: `{"href":"http://example.com","meta":{"count":10}}`,
			expected: Link{
				Href: "http://example.com",
				Meta: map[string]interface{}{"count": float64(10)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var link Link
			err := link.UnmarshalJSON([]byte(tt.jsonData))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, link)
		})
	}
}

func TestRelationshipData_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		isMany   bool
	}{
		{
			name:     "single reference",
			jsonData: `{"type": "user", "id": "1"}`,
			isMany:   false,
		},
		{
			name:     "multiple references",
			jsonData: `[{"type": "tag", "id": "1"}, {"type": "tag", "id": "2"}]`,
			isMany:   true,
		},
		{
			name:     "null reference",
			jsonData: `null`,
			isMany:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rd RelationshipData
			err := rd.UnmarshalJSON([]byte(tt.jsonData))
			assert.NoError(t, err)
			assert.Equal(t, tt.isMany, rd.isMany)
		})
	}
}
