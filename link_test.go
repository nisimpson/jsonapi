package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
