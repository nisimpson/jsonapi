package jsonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expected  int64
		expectErr bool
	}{
		{
			name:     "int",
			value:    int(42),
			expected: 42,
		},
		{
			name:     "int8",
			value:    int8(8),
			expected: 8,
		},
		{
			name:     "int16",
			value:    int16(16),
			expected: 16,
		},
		{
			name:     "int32",
			value:    int32(32),
			expected: 32,
		},
		{
			name:     "int64",
			value:    int64(64),
			expected: 64,
		},
		{
			name:     "uint",
			value:    uint(42),
			expected: 42,
		},
		{
			name:     "uint8",
			value:    uint8(8),
			expected: 8,
		},
		{
			name:     "uint16",
			value:    uint16(16),
			expected: 16,
		},
		{
			name:     "uint32",
			value:    uint32(32),
			expected: 32,
		},
		{
			name:     "uint64",
			value:    uint64(64),
			expected: 64,
		},
		{
			name:     "float32",
			value:    float32(42.0),
			expected: 42,
		},
		{
			name:     "float64",
			value:    float64(42.0),
			expected: 42,
		},
		{
			name:     "string valid",
			value:    "42",
			expected: 42,
		},
		{
			name:      "string invalid",
			value:     "not a number",
			expectErr: true,
		},
		{
			name:      "bool",
			value:     true,
			expectErr: true,
		},
		{
			name:      "nil",
			value:     nil,
			expectErr: true,
		},
		{
			name:      "struct",
			value:     struct{}{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToInt(tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToUint(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expected  uint64
		expectErr bool
	}{
		{
			name:     "uint",
			value:    uint(42),
			expected: 42,
		},
		{
			name:     "uint8",
			value:    uint8(8),
			expected: 8,
		},
		{
			name:     "uint16",
			value:    uint16(16),
			expected: 16,
		},
		{
			name:     "uint32",
			value:    uint32(32),
			expected: 32,
		},
		{
			name:     "uint64",
			value:    uint64(64),
			expected: 64,
		},
		{
			name:     "int positive",
			value:    int(42),
			expected: 42,
		},
		// The implementation doesn't check for negative values
		{
			name:     "int negative",
			value:    int(-42),
			expected: 18446744073709551574, // This is what uint64(-42) evaluates to
		},
		{
			name:     "int8 positive",
			value:    int8(8),
			expected: 8,
		},
		{
			name:     "int16 positive",
			value:    int16(16),
			expected: 16,
		},
		{
			name:     "int32 positive",
			value:    int32(32),
			expected: 32,
		},
		{
			name:     "int64 positive",
			value:    int64(64),
			expected: 64,
		},
		{
			name:     "float32 positive",
			value:    float32(42.0),
			expected: 42,
		},
		// The implementation doesn't check for negative values in float32
		{
			name:     "float32 negative",
			value:    float32(-42.0),
			expected: 0, // Float32 to uint64 conversion with negative value gives 0
		},
		{
			name:     "float64 positive",
			value:    float64(42.0),
			expected: 42,
		},
		{
			name:     "string valid",
			value:    "42",
			expected: 42,
		},
		{
			name:      "string negative",
			value:     "-42",
			expectErr: true, // ParseUint will catch this
		},
		{
			name:      "string invalid",
			value:     "not a number",
			expectErr: true,
		},
		{
			name:      "bool",
			value:     true,
			expectErr: true,
		},
		{
			name:      "nil",
			value:     nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToUint(tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToFloat(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expected  float64
		expectErr bool
	}{
		{
			name:     "float32",
			value:    float32(42.5),
			expected: 42.5,
		},
		{
			name:     "float64",
			value:    float64(42.5),
			expected: 42.5,
		},
		{
			name:     "int",
			value:    int(42),
			expected: 42.0,
		},
		{
			name:     "int8",
			value:    int8(8),
			expected: 8.0,
		},
		{
			name:     "int16",
			value:    int16(16),
			expected: 16.0,
		},
		{
			name:     "int32",
			value:    int32(32),
			expected: 32.0,
		},
		{
			name:     "int64",
			value:    int64(64),
			expected: 64.0,
		},
		{
			name:     "uint",
			value:    uint(42),
			expected: 42.0,
		},
		{
			name:     "uint8",
			value:    uint8(8),
			expected: 8.0,
		},
		{
			name:     "uint16",
			value:    uint16(16),
			expected: 16.0,
		},
		{
			name:     "uint32",
			value:    uint32(32),
			expected: 32.0,
		},
		{
			name:     "uint64",
			value:    uint64(64),
			expected: 64.0,
		},
		{
			name:     "string valid integer",
			value:    "42",
			expected: 42.0,
		},
		{
			name:     "string valid float",
			value:    "42.5",
			expected: 42.5,
		},
		{
			name:      "string invalid",
			value:     "not a number",
			expectErr: true,
		},
		{
			name:      "bool",
			value:     true,
			expectErr: true,
		},
		{
			name:      "nil",
			value:     nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToFloat(tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToBool(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expected  bool
		expectErr bool
	}{
		{
			name:     "bool true",
			value:    true,
			expected: true,
		},
		{
			name:     "bool false",
			value:    false,
			expected: false,
		},
		{
			name:     "int 1",
			value:    int(1),
			expected: true,
		},
		{
			name:     "int 0",
			value:    int(0),
			expected: false,
		},
		// The implementation treats any non-zero value as true
		{
			name:     "int other",
			value:    int(42),
			expected: true,
		},
		{
			name:     "int8 1",
			value:    int8(1),
			expected: true,
		},
		{
			name:     "int8 0",
			value:    int8(0),
			expected: false,
		},
		{
			name:     "int16 1",
			value:    int16(1),
			expected: true,
		},
		{
			name:     "int32 1",
			value:    int32(1),
			expected: true,
		},
		{
			name:     "int64 1",
			value:    int64(1),
			expected: true,
		},
		{
			name:     "uint 1",
			value:    uint(1),
			expected: true,
		},
		{
			name:     "uint8 1",
			value:    uint8(1),
			expected: true,
		},
		{
			name:     "uint16 1",
			value:    uint16(1),
			expected: true,
		},
		{
			name:     "uint32 1",
			value:    uint32(1),
			expected: true,
		},
		{
			name:     "uint64 1",
			value:    uint64(1),
			expected: true,
		},
		{
			name:     "float32 1.0",
			value:    float32(1.0),
			expected: true,
		},
		// The implementation treats any non-zero value as true
		{
			name:     "float64 other",
			value:    float64(42.5),
			expected: true,
		},
		{
			name:     "string true",
			value:    "true",
			expected: true,
		},
		{
			name:     "string TRUE",
			value:    "TRUE",
			expected: true,
		},
		{
			name:     "string t",
			value:    "t",
			expected: true,
		},
		{
			name:     "string T",
			value:    "T",
			expected: true,
		},
		{
			name:     "string 1",
			value:    "1",
			expected: true,
		},
		{
			name:     "string false",
			value:    "false",
			expected: false,
		},
		{
			name:     "string FALSE",
			value:    "FALSE",
			expected: false,
		},
		{
			name:     "string f",
			value:    "f",
			expected: false,
		},
		{
			name:     "string F",
			value:    "F",
			expected: false,
		},
		{
			name:     "string 0",
			value:    "0",
			expected: false,
		},
		{
			name:      "string invalid",
			value:     "not a bool",
			expectErr: true,
		},
		{
			name:      "nil",
			value:     nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToBool(tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
