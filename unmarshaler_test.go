package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalMany(t *testing.T) {
	jsonData := `{
		"data": [
			{"type": "test", "id": "1", "attributes": {"name": "test1"}},
			{"type": "test", "id": "2", "attributes": {"name": "test2"}}
		]
	}`

	var resources []testResource
	err := Unmarshal([]byte(jsonData), &resources)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
	if len(resources) >= 2 {
		assert.Equal(t, "1", resources[0].ID)
		assert.Equal(t, "test1", resources[0].Name)
		assert.Equal(t, "2", resources[1].ID)
		assert.Equal(t, "test2", resources[1].Name)
	}
}

func TestDocument_UnmarshalData_Single(t *testing.T) {
	jsonData := `{
		"data": {"type": "test", "id": "1", "attributes": {"name": "test1"}}
	}`

	var doc Document
	err := json.Unmarshal([]byte(jsonData), &doc)
	assert.NoError(t, err)

	var resource testResource
	err = doc.UnmarshalData(&resource)
	assert.NoError(t, err)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "test1", resource.Name)
}

func TestDocument_UnmarshalData_Many(t *testing.T) {
	jsonData := `{
		"data": [
			{"type": "test", "id": "1", "attributes": {"name": "test1"}},
			{"type": "test", "id": "2", "attributes": {"name": "test2"}}
		]
	}`

	var doc Document
	err := json.Unmarshal([]byte(jsonData), &doc)
	assert.NoError(t, err)

	var resources []testResource
	err = doc.UnmarshalData(&resources)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
}
