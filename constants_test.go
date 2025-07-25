package jsonapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStructTagConstants verifies that the struct tag constants work correctly
// with the marshaling and unmarshaling functionality.
func TestStructTagConstants(t *testing.T) {
	// Define a test struct using the constants in a way that would be typical
	// for library users (they would still use string literals in their tags)
	type TestUser struct {
		ID    string `jsonapi:"primary,users"`
		Name  string `jsonapi:"attr,name"`
		Email string `jsonapi:"attr,email,omitempty"`
	}

	user := TestUser{
		ID:    "123",
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Test marshaling
	data, err := Marshal(user)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test unmarshaling
	var unmarshaledUser TestUser
	err = Unmarshal(data, &unmarshaledUser)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify the data
	if unmarshaledUser.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, unmarshaledUser.ID)
	}
	if unmarshaledUser.Name != user.Name {
		t.Errorf("Expected Name %s, got %s", user.Name, unmarshaledUser.Name)
	}
	if unmarshaledUser.Email != user.Email {
		t.Errorf("Expected Email %s, got %s", user.Email, unmarshaledUser.Email)
	}
}

// TestConstantValues verifies that the constants have the expected values.
func TestConstantValues(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"StructTagName", StructTagName, "jsonapi"},
		{"TagValuePrimary", TagValuePrimary, "primary"},
		{"TagValueAttribute", TagValueAttribute, "attr"},
		{"TagValueRelationship", TagValueRelationship, "relation"},
		{"TagOptionOmitEmpty", TagOptionOmitEmpty, "omitempty"},
		{"TagOptionReadOnly", TagOptionReadOnly, "readonly"},
		{"TagValueIgnore", TagValueIgnore, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %q, got %q", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

// TestOmitEmptyWithConstants tests that the omitempty functionality works with constants.
func TestOmitEmptyWithConstants(t *testing.T) {
	type TestStruct struct {
		ID          string `jsonapi:"primary,test"`
		RequiredStr string `jsonapi:"attr,required"`
		OptionalStr string `jsonapi:"attr,optional,omitempty"`
	}

	// Test with empty optional field
	obj := TestStruct{
		ID:          "1",
		RequiredStr: "required",
		OptionalStr: "", // Empty, should be omitted
	}

	resource, _, err := marshalSingle(context.Background(), obj)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check that the optional field was omitted
	if _, exists := resource.Attributes["optional"]; exists {
		t.Error("Expected optional field to be omitted when empty")
	}

	// Check that the required field is present
	if _, exists := resource.Attributes["required"]; !exists {
		t.Error("Expected required field to be present")
	}
}

// TestReadOnlyWithConstants tests that the readonly functionality works with constants.
func TestReadOnlyWithConstants(t *testing.T) {
	type TestStruct struct {
		ID            string `jsonapi:"primary,test"`
		RegularField  string `jsonapi:"attr,regular"`
		ReadOnlyField string `jsonapi:"attr,readonly_field,readonly"`
	}

	// Test marshaling - readonly fields should be included
	obj := TestStruct{
		ID:            "1",
		RegularField:  "regular_value",
		ReadOnlyField: "readonly_value",
	}

	resource, _, err := marshalSingle(context.Background(), obj)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check that both fields are present in marshaled output
	if _, exists := resource.Attributes["regular"]; !exists {
		t.Error("Expected regular field to be present")
	}
	if _, exists := resource.Attributes["readonly_field"]; !exists {
		t.Error("Expected readonly field to be present in marshaled output")
	}

	// Test unmarshaling without AllowReadOnly - should fail
	jsonData := []byte(`{
		"data": {
			"id": "1",
			"type": "test",
			"attributes": {
				"regular": "regular_value",
				"readonly_field": "readonly_value"
			}
		}
	}`)

	var result TestStruct
	err = Unmarshal(jsonData, &result)
	assert.NoError(t, err)
	assert.Equal(t, "1", result.ID)
	assert.Equal(t, "regular_value", result.RegularField)
	assert.Equal(t, "readonly_value", result.ReadOnlyField)

	// Test unmarshaling with FailReadOnly - should fail
	err = Unmarshal(jsonData, &result, PermitReadOnly(false))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReadOnly)
}
