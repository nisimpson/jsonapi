package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      Error
		expected string
	}{
		{
			name: "with code, title, and detail",
			err: Error{
				Code:   "123",
				Title:  "Test Error",
				Detail: "This is a test error",
			},
			expected: "Test Error: This is a test error (123)",
		},
		{
			name: "with title and detail only",
			err: Error{
				Title:  "Test Error",
				Detail: "This is a test error",
			},
			expected: "Test Error: This is a test error",
		},
		{
			name: "with detail only",
			err: Error{
				Detail: "This is a test error",
			},
			expected: "This is a test error",
		},
		{
			name: "with all fields",
			err: Error{
				ID:     "error-1",
				Status: "400",
				Code:   "123",
				Title:  "Test Error",
				Detail: "This is a test error",
				Source: map[string]interface{}{
					"pointer": "/data/attributes/title",
				},
				Links: map[string]interface{}{
					"about": "https://example.com/errors/123",
				},
			},
			expected: "Test Error: This is a test error (123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestMultiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errs     MultiError
		expected string
		panics   bool
	}{
		{
			name: "single error",
			errs: MultiError{
				{
					Title:  "Test Error",
					Detail: "This is a test error",
				},
			},
			expected: "Test Error: This is a test error",
		},
		{
			name: "multiple errors",
			errs: MultiError{
				{
					Title:  "Error 1",
					Detail: "This is error 1",
				},
				{
					Title:  "Error 2",
					Detail: "This is error 2",
				},
			},
			expected: "Error 1: This is error 1\nError 2: This is error 2",
		},
		{
			name:   "empty error",
			errs:   MultiError{},
			panics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panics {
				assert.Panics(t, func() {
					_ = tt.errs.Error()
				})
			} else {
				assert.Equal(t, tt.expected, tt.errs.Error())
			}
		})
	}
}
