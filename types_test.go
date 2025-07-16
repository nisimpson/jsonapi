package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResource_Ref(t *testing.T) {
	resource := Resource{
		ID:   "1",
		Type: "users",
		Attributes: map[string]interface{}{
			"name": "John Doe",
		},
		Relationships: map[string]Relationship{
			"posts": {
				Data: MultiResource(Resource{ID: "1", Type: "posts"}),
			},
		},
		Meta: map[string]interface{}{
			"version": "1.0",
		},
		Links: map[string]Link{
			"self": {Href: "/users/1"},
		},
	}

	ref := resource.Ref()

	assert.Equal(t, "1", ref.ID)
	assert.Equal(t, "users", ref.Type)
	assert.Nil(t, ref.Attributes)
	assert.Nil(t, ref.Relationships)
	assert.Nil(t, ref.Meta)
	assert.Nil(t, ref.Links)
}

func TestPrimaryData_SingleResource(t *testing.T) {
	resource := Resource{ID: "1", Type: "users"}
	data := SingleResource(resource)

	assert.False(t, data.Null())

	single, ok := data.One()
	assert.True(t, ok)
	assert.Equal(t, resource, single)

	many, ok := data.Many()
	assert.False(t, ok)
	assert.Nil(t, many)
}

func TestPrimaryData_MultiResource(t *testing.T) {
	resources := []Resource{
		{ID: "1", Type: "users"},
		{ID: "2", Type: "users"},
	}
	data := MultiResource(resources...)

	assert.False(t, data.Null())

	single, ok := data.One()
	assert.False(t, ok)
	assert.Equal(t, Resource{}, single)

	many, ok := data.Many()
	assert.True(t, ok)
	assert.Equal(t, resources, many)
}

func TestPrimaryData_NullResource(t *testing.T) {
	data := NullResource()

	assert.True(t, data.Null())

	single, ok := data.One()
	assert.False(t, ok)
	assert.Equal(t, Resource{}, single)

	many, ok := data.Many()
	assert.False(t, ok)
	assert.Nil(t, many)
}

func TestPrimaryData_Iter(t *testing.T) {
	t.Run("single resource", func(t *testing.T) {
		resource := Resource{ID: "1", Type: "users"}
		data := SingleResource(resource)

		var collected []Resource
		for r := range data.Iter() {
			collected = append(collected, r)
		}

		assert.Len(t, collected, 1)
		assert.Equal(t, resource, collected[0])
	})

	t.Run("multiple resources", func(t *testing.T) {
		resources := []Resource{
			{ID: "1", Type: "users"},
			{ID: "2", Type: "users"},
		}
		data := MultiResource(resources...)

		var collected []Resource
		for r := range data.Iter() {
			collected = append(collected, r)
		}

		assert.Len(t, collected, 2)
		assert.Equal(t, resources, collected)
	})

	t.Run("null resource", func(t *testing.T) {
		data := NullResource()

		var collected []Resource
		for r := range data.Iter() {
			collected = append(collected, r)
		}

		assert.Len(t, collected, 0)
	})
}

func TestPrimaryData_JSON(t *testing.T) {
	t.Run("marshal single resource", func(t *testing.T) {
		resource := Resource{ID: "1", Type: "users"}
		data := SingleResource(resource)

		jsonData, err := json.Marshal(data)
		require.NoError(t, err)

		expected := `{"id":"1","type":"users"}`
		assert.JSONEq(t, expected, string(jsonData))
	})

	t.Run("marshal multiple resources", func(t *testing.T) {
		resources := []Resource{
			{ID: "1", Type: "users"},
			{ID: "2", Type: "users"},
		}
		data := MultiResource(resources...)

		jsonData, err := json.Marshal(data)
		require.NoError(t, err)

		expected := `[{"id":"1","type":"users"},{"id":"2","type":"users"}]`
		assert.JSONEq(t, expected, string(jsonData))
	})

	t.Run("marshal null resource", func(t *testing.T) {
		data := NullResource()

		jsonData, err := json.Marshal(data)
		require.NoError(t, err)

		assert.Equal(t, "null", string(jsonData))
	})

	t.Run("unmarshal single resource", func(t *testing.T) {
		jsonData := `{"id":"1","type":"users","attributes":{"name":"John"}}`

		var data PrimaryData
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		assert.False(t, data.Null())
		resource, ok := data.One()
		assert.True(t, ok)
		assert.Equal(t, "1", resource.ID)
		assert.Equal(t, "users", resource.Type)
		assert.Equal(t, "John", resource.Attributes["name"])
	})

	t.Run("unmarshal multiple resources", func(t *testing.T) {
		jsonData := `[{"id":"1","type":"users"},{"id":"2","type":"users"}]`

		var data PrimaryData
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		assert.False(t, data.Null())
		resources, ok := data.Many()
		assert.True(t, ok)
		assert.Len(t, resources, 2)
		assert.Equal(t, "1", resources[0].ID)
		assert.Equal(t, "2", resources[1].ID)
	})

	t.Run("unmarshal null resource", func(t *testing.T) {
		jsonData := `null`

		var data PrimaryData
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		assert.True(t, data.Null())
	})
}

func TestDocument_Structure(t *testing.T) {
	doc := Document{
		Meta: map[string]interface{}{
			"version": "1.0",
		},
		Data: SingleResource(Resource{
			ID:   "1",
			Type: "users",
			Attributes: map[string]interface{}{
				"name": "John Doe",
			},
		}),
		Links: map[string]Link{
			"self": {Href: "/users/1"},
		},
		Included: []Resource{
			{ID: "1", Type: "posts", Attributes: map[string]interface{}{"title": "Post 1"}},
		},
	}

	jsonData, err := json.Marshal(doc)
	require.NoError(t, err)

	var unmarshaled Document
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, doc.Meta, unmarshaled.Meta)
	assert.Equal(t, doc.Links, unmarshaled.Links)
	assert.Len(t, unmarshaled.Included, 1)

	resource, ok := unmarshaled.Data.One()
	assert.True(t, ok)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
}

func TestError_Structure(t *testing.T) {
	err := Error{
		ID:     "error-1",
		Status: "400",
		Code:   "INVALID_REQUEST",
		Title:  "Invalid Request",
		Detail: "The request is invalid",
		Source: map[string]interface{}{
			"pointer": "/data/attributes/name",
		},
		Links: map[string]interface{}{
			"about": "https://example.com/errors/invalid-request",
		},
	}

	jsonData, jsonErr := json.Marshal(err)
	require.NoError(t, jsonErr)

	var unmarshaled Error
	jsonErr = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, jsonErr)

	assert.Equal(t, err, unmarshaled)
}

func TestLink_Structure(t *testing.T) {
	link := Link{
		Href: "https://example.com/users/1",
		Meta: map[string]interface{}{
			"count": 10,
		},
	}

	jsonData, err := json.Marshal(link)
	require.NoError(t, err)

	var unmarshaled Link
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, link.Href, unmarshaled.Href)
	assert.Equal(t, float64(10), unmarshaled.Meta["count"]) // JSON numbers are float64

	jsonData = []byte("null")
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "", unmarshaled.Href)
	assert.Len(t, unmarshaled.Meta, 0)
}

func TestRelationship_Structure(t *testing.T) {
	rel := Relationship{
		Meta: map[string]interface{}{
			"count": 2,
		},
		Links: map[string]Link{
			"self": {Href: "/users/1/relationships/posts"},
		},
		Data: MultiResource(
			Resource{ID: "1", Type: "posts"},
			Resource{ID: "2", Type: "posts"},
		),
	}

	jsonData, err := json.Marshal(rel)
	require.NoError(t, err)

	var unmarshaled Relationship
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, float64(2), unmarshaled.Meta["count"]) // JSON numbers are float64
	assert.Equal(t, rel.Links, unmarshaled.Links)

	resources, ok := unmarshaled.Data.Many()
	assert.True(t, ok)
	assert.Len(t, resources, 2)
}
