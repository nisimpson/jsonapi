package jsonapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testResourceWithRel struct {
	ID   string `jsonapi:"primary,test"`
	Name string `jsonapi:"attr,name"`
}

func (t testResourceWithRel) ResourceID() string   { return t.ID }
func (t testResourceWithRel) ResourceType() string { return "test" }
func (t testResourceWithRel) Relationships() map[string]RelationType {
	return map[string]RelationType{"author": RelationToOne}
}
func (t testResourceWithRel) MarshalRef(name string) []ResourceIdentifier {
	return nil
}

func TestWithTopMeta(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopMeta("total", 42))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, float64(42), doc.Meta["total"])
}

func TestWithTopLink(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopLink("self", Link{Href: "http://example.com/test/1"}))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/test/1", doc.Links["self"].Href)
}

func TestWithTopHref(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopHref("self", "http://example.com/test/1"))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/test/1", doc.Links["self"].Href)
}

func TestWithMaxIncludeDepth(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	// This should not error even with depth limit
	data, err := Marshal(resource, WithMaxIncludeDepth(1))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithTypeValidation(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTypeValidation())
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithLinkResolver(t *testing.T) {
	resolver := &testLinkResolver{}
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithLinkResolver("self", resolver))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithDefaultLinks(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithDefaultLinks("http://example.com"))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

type testLinkResolver struct{}

func (r *testLinkResolver) ResolveResourceLink(key string, id ResourceIdentifier) (Link, bool) {
	if key == "self" {
		return Link{Href: "http://example.com/" + id.ResourceType() + "/" + id.ResourceID()}, true
	}
	return Link{}, false
}

func (r *testLinkResolver) ResolveRelationshipLink(key string, name string, id RelationshipMarshaler) (Link, bool) {
	return Link{}, false
}

func TestSelfLinkResolver_ResolveResourceLink(t *testing.T) {
	resolver := SelfLinkResolver{
		BaseURL:             "http://example.com",
		SelfResourcePattern: "%s/%s/%s",
	}
	resource := testResource{ID: "1", Name: "test"}

	link, ok := resolver.ResolveResourceLink("self", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1", link.Href)
}

func TestSelfLinkResolver_ResolveRelationshipLink(t *testing.T) {
	resolver := SelfLinkResolver{
		BaseURL:                    "http://example.com",
		SelfRelationshipPattern:    "%s/%s/%s/relationships/%s",
		RelatedRelationshipPattern: "%s/%s/%s/%s",
	}
	resource := testResourceWithRel{ID: "1", Name: "test"}

	link, ok := resolver.ResolveRelationshipLink("self", "author", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1/relationships/author", link.Href)

	link, ok = resolver.ResolveRelationshipLink("related", "author", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1/author", link.Href)
}
