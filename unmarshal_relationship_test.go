package jsonapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for relationship unmarshaling
type RelationshipParent struct {
	ID           string                `jsonapi:"primary,parents"`
	Name         string                `jsonapi:"attr,name"`
	SingleChild  RelationshipChild     `jsonapi:"relation,single_child"`
	ManyChildren []RelationshipChild   `jsonapi:"relation,many_children"`
	SinglePtr    *RelationshipChild    `jsonapi:"relation,single_ptr"`
	ManyPtrs     []*RelationshipChild  `jsonapi:"relation,many_ptrs"`
	EmptyRel     *RelationshipChild    `jsonapi:"relation,empty_rel"`
	NullRel      *RelationshipChild    `jsonapi:"relation,null_rel"`
	ManyEmpty    []RelationshipChild   `jsonapi:"relation,many_empty"`
	ManyNull     []RelationshipChild   `jsonapi:"relation,many_null"`
	CustomRel    RelationshipCustom    `jsonapi:"relation,custom_rel"`
	CustomPtr    *RelationshipCustom   `jsonapi:"relation,custom_ptr"`
	ManyCustom   []RelationshipCustom  `jsonapi:"relation,many_custom"`
	ManyCustomPtr []*RelationshipCustom `jsonapi:"relation,many_custom_ptr"`
}

type RelationshipChild struct {
	ID     string `jsonapi:"primary,children"`
	Name   string `jsonapi:"attr,name"`
	Age    int    `jsonapi:"attr,age"`
	Active bool   `jsonapi:"attr,active"`
}

type RelationshipCustom struct {
	ID     string `jsonapi:"primary,custom"`
	Value  string
}

// Custom unmarshaler implementation
func (c *RelationshipCustom) UnmarshalJSONAPIResource(ctx context.Context, resource Resource) error {
	c.ID = resource.ID
	c.Value = "custom-" + resource.ID
	return nil
}

func TestUnmarshalRelationship_ToOne(t *testing.T) {
	// Test unmarshaling a to-one relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"single_child": {
					"data": {
						"id": "child-1",
						"type": "children"
					}
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	assert.Equal(t, "child-1", parent.SingleChild.ID)
	assert.Equal(t, "Child 1", parent.SingleChild.Name)
	assert.Equal(t, 10, parent.SingleChild.Age)
	assert.True(t, parent.SingleChild.Active)
}

func TestUnmarshalRelationship_ToOnePointer(t *testing.T) {
	// Test unmarshaling a to-one pointer relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"single_ptr": {
					"data": {
						"id": "child-1",
						"type": "children"
					}
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	require.NotNil(t, parent.SinglePtr)
	assert.Equal(t, "child-1", parent.SinglePtr.ID)
	assert.Equal(t, "Child 1", parent.SinglePtr.Name)
	assert.Equal(t, 10, parent.SinglePtr.Age)
	assert.True(t, parent.SinglePtr.Active)
}

func TestUnmarshalRelationship_ToMany(t *testing.T) {
	// Test unmarshaling a to-many relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_children": {
					"data": [
						{
							"id": "child-1",
							"type": "children"
						},
						{
							"id": "child-2",
							"type": "children"
						}
					]
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			},
			{
				"id": "child-2",
				"type": "children",
				"attributes": {
					"name": "Child 2",
					"age": 8,
					"active": false
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	require.Len(t, parent.ManyChildren, 2)
	assert.Equal(t, "child-1", parent.ManyChildren[0].ID)
	assert.Equal(t, "Child 1", parent.ManyChildren[0].Name)
	assert.Equal(t, 10, parent.ManyChildren[0].Age)
	assert.True(t, parent.ManyChildren[0].Active)
	assert.Equal(t, "child-2", parent.ManyChildren[1].ID)
	assert.Equal(t, "Child 2", parent.ManyChildren[1].Name)
	assert.Equal(t, 8, parent.ManyChildren[1].Age)
	assert.False(t, parent.ManyChildren[1].Active)
}

func TestUnmarshalRelationship_ToManyPointers(t *testing.T) {
	// Test unmarshaling a to-many pointers relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_ptrs": {
					"data": [
						{
							"id": "child-1",
							"type": "children"
						},
						{
							"id": "child-2",
							"type": "children"
						}
					]
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			},
			{
				"id": "child-2",
				"type": "children",
				"attributes": {
					"name": "Child 2",
					"age": 8,
					"active": false
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated
	require.Len(t, parent.ManyPtrs, 2)
	require.NotNil(t, parent.ManyPtrs[0])
	assert.Equal(t, "child-1", parent.ManyPtrs[0].ID)
	assert.Equal(t, "Child 1", parent.ManyPtrs[0].Name)
	assert.Equal(t, 10, parent.ManyPtrs[0].Age)
	assert.True(t, parent.ManyPtrs[0].Active)
	require.NotNil(t, parent.ManyPtrs[1])
	assert.Equal(t, "child-2", parent.ManyPtrs[1].ID)
	assert.Equal(t, "Child 2", parent.ManyPtrs[1].Name)
	assert.Equal(t, 8, parent.ManyPtrs[1].Age)
	assert.False(t, parent.ManyPtrs[1].Active)
}

func TestUnmarshalRelationship_NullRelationship(t *testing.T) {
	// Test unmarshaling a null relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"null_rel": {
					"data": null
				}
			}
		}
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent)
	require.NoError(t, err)

	// Verify the relationship is nil
	assert.Nil(t, parent.NullRel)
}

func TestUnmarshalRelationship_EmptyRelationship(t *testing.T) {
	// Test unmarshaling an empty relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"empty_rel": {
					"links": {
						"self": "/api/parents/1/relationships/empty_rel",
						"related": "/api/parents/1/empty_rel"
					}
				}
			}
		}
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent)
	require.NoError(t, err)

	// Verify the relationship is nil (no data provided)
	assert.Nil(t, parent.EmptyRel)
}

func TestUnmarshalRelationship_EmptyToMany(t *testing.T) {
	// Test unmarshaling an empty to-many relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_empty": {
					"data": []
				}
			}
		}
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent)
	require.NoError(t, err)

	// Verify the relationship is an empty slice
	assert.Empty(t, parent.ManyEmpty)
}

func TestUnmarshalRelationship_NullToMany(t *testing.T) {
	// Test unmarshaling a null to-many relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_null": {
					"data": null
				}
			}
		}
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent)
	require.NoError(t, err)

	// Verify the relationship is nil
	assert.Nil(t, parent.ManyNull)
}

func TestUnmarshalRelationship_CustomUnmarshaler(t *testing.T) {
	// Test unmarshaling with a custom unmarshaler
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"custom_rel": {
					"data": {
						"id": "custom-1",
						"type": "custom"
					}
				},
				"custom_ptr": {
					"data": {
						"id": "custom-2",
						"type": "custom"
					}
				},
				"many_custom": {
					"data": [
						{
							"id": "custom-3",
							"type": "custom"
						},
						{
							"id": "custom-4",
							"type": "custom"
						}
					]
				},
				"many_custom_ptr": {
					"data": [
						{
							"id": "custom-5",
							"type": "custom"
						},
						{
							"id": "custom-6",
							"type": "custom"
						}
					]
				}
			}
		}
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent)
	require.NoError(t, err)

	// Verify custom unmarshaler was used
	assert.Equal(t, "custom-1", parent.CustomRel.ID)
	assert.Equal(t, "custom-custom-1", parent.CustomRel.Value)
	
	require.NotNil(t, parent.CustomPtr)
	assert.Equal(t, "custom-2", parent.CustomPtr.ID)
	assert.Equal(t, "custom-custom-2", parent.CustomPtr.Value)
	
	require.Len(t, parent.ManyCustom, 2)
	assert.Equal(t, "custom-3", parent.ManyCustom[0].ID)
	assert.Equal(t, "custom-custom-3", parent.ManyCustom[0].Value)
	assert.Equal(t, "custom-4", parent.ManyCustom[1].ID)
	assert.Equal(t, "custom-custom-4", parent.ManyCustom[1].Value)
	
	require.Len(t, parent.ManyCustomPtr, 2)
	require.NotNil(t, parent.ManyCustomPtr[0])
	assert.Equal(t, "custom-5", parent.ManyCustomPtr[0].ID)
	assert.Equal(t, "custom-custom-5", parent.ManyCustomPtr[0].Value)
	require.NotNil(t, parent.ManyCustomPtr[1])
	assert.Equal(t, "custom-6", parent.ManyCustomPtr[1].ID)
	assert.Equal(t, "custom-custom-6", parent.ManyCustomPtr[1].Value)
}

func TestUnmarshalRelationship_SingleInMany(t *testing.T) {
	// Test unmarshaling a single resource in a to-many relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"many_children": {
					"data": {
						"id": "child-1",
						"type": "children"
					}
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify the relationship was populated as a slice with one element
	require.Len(t, parent.ManyChildren, 1)
	assert.Equal(t, "child-1", parent.ManyChildren[0].ID)
	assert.Equal(t, "Child 1", parent.ManyChildren[0].Name)
	assert.Equal(t, 10, parent.ManyChildren[0].Age)
	assert.True(t, parent.ManyChildren[0].Active)
}

func TestUnmarshalRelationship_ManyInOne(t *testing.T) {
	// Test unmarshaling a to-many resource in a to-one relationship
	jsonData := []byte(`{
		"data": {
			"id": "parent-1",
			"type": "parents",
			"attributes": {
				"name": "Parent 1"
			},
			"relationships": {
				"single_child": {
					"data": [
						{
							"id": "child-1",
							"type": "children"
						},
						{
							"id": "child-2",
							"type": "children"
						}
					]
				}
			}
		},
		"included": [
			{
				"id": "child-1",
				"type": "children",
				"attributes": {
					"name": "Child 1",
					"age": 10,
					"active": true
				}
			},
			{
				"id": "child-2",
				"type": "children",
				"attributes": {
					"name": "Child 2",
					"age": 8,
					"active": false
				}
			}
		]
	}`)

	var parent RelationshipParent
	err := Unmarshal(jsonData, &parent, PopulateFromIncluded())
	require.NoError(t, err)

	// Verify only the first resource was used
	assert.Equal(t, "child-1", parent.SingleChild.ID)
	assert.Equal(t, "Child 1", parent.SingleChild.Name)
	assert.Equal(t, 10, parent.SingleChild.Age)
	assert.True(t, parent.SingleChild.Active)
}

func TestFindIncludedResource(t *testing.T) {
	included := []Resource{
		{
			ID:   "1",
			Type: "users",
			Attributes: map[string]interface{}{
				"name": "User 1",
			},
		},
		{
			ID:   "2",
			Type: "posts",
			Attributes: map[string]interface{}{
				"title": "Post 2",
			},
		},
		{
			ID:   "3",
			Type: "users",
			Attributes: map[string]interface{}{
				"name": "User 3",
			},
		},
	}

	// Test finding an existing resource
	resource := findIncludedResource("1", "users", included)
	assert.NotNil(t, resource)
	assert.Equal(t, "1", resource.ID)
	assert.Equal(t, "users", resource.Type)
	assert.Equal(t, "User 1", resource.Attributes["name"])

	// Test finding another existing resource
	resource = findIncludedResource("2", "posts", included)
	assert.NotNil(t, resource)
	assert.Equal(t, "2", resource.ID)
	assert.Equal(t, "posts", resource.Type)
	assert.Equal(t, "Post 2", resource.Attributes["title"])

	// Test finding a non-existent ID
	resource = findIncludedResource("4", "users", included)
	assert.Nil(t, resource)

	// Test finding a non-existent type
	resource = findIncludedResource("1", "posts", included)
	assert.Nil(t, resource)
}
