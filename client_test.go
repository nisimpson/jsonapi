package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

// Feature: jsonapi-http-client, Property 1: DefaultURLResolver URL pattern correctness
// Validates: Requirements 2.8, 2.9, 2.10, 2.11, 2.12, 2.13, 2.14
func TestDefaultURLResolver_URLPatternCorrectness(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("ResolveResourceURL produces baseURL/type/id", func(t *testing.T) {
		f := func(baseURL, resourceType, id string) bool {
			resolver := DefaultURLResolver{BaseURL: baseURL}
			got := resolver.ResolveResourceURL(resourceType, id)
			expected := baseURL + "/" + resourceType + "/" + id
			return got == expected
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("ResolveCollectionURL produces baseURL/type", func(t *testing.T) {
		f := func(baseURL, resourceType string) bool {
			resolver := DefaultURLResolver{BaseURL: baseURL}
			got := resolver.ResolveCollectionURL(resourceType)
			expected := baseURL + "/" + resourceType
			return got == expected
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("ResolveRelationshipURL produces baseURL/type/id/relationships/relationship", func(t *testing.T) {
		f := func(baseURL, resourceType, id, relationship string) bool {
			resolver := DefaultURLResolver{BaseURL: baseURL}
			got := resolver.ResolveRelationshipURL(resourceType, id, relationship)
			expected := baseURL + "/" + resourceType + "/" + id + "/relationships/" + relationship
			return got == expected
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("ResolveRelatedURL produces baseURL/type/id/relationship", func(t *testing.T) {
		f := func(baseURL, resourceType, id, relationship string) bool {
			resolver := DefaultURLResolver{BaseURL: baseURL}
			got := resolver.ResolveRelatedURL(resourceType, id, relationship)
			expected := baseURL + "/" + resourceType + "/" + id + "/" + relationship
			return got == expected
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Task 1.3: Unit tests for NewClient defaults and custom options
// Requirements: 1.1, 1.2, 1.3, 1.6, 1.7

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	c := NewClient("http://example.com")
	assert.Equal(t, http.DefaultClient, c.httpClient)
}

func TestNewClient_DefaultResolver(t *testing.T) {
	c := NewClient("http://example.com")
	expected := DefaultURLResolver{BaseURL: "http://example.com"}
	assert.Equal(t, expected, c.resolver)
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := NewClient("http://example.com", WithHTTPClient(custom))
	assert.Same(t, custom, c.httpClient)
}

func TestNewClient_WithURLResolver(t *testing.T) {
	custom := DefaultURLResolver{BaseURL: "http://custom.com"}
	c := NewClient("http://example.com", WithURLResolver(custom))
	assert.Equal(t, custom, c.resolver)
}

func TestNewClient_WithRequestMiddleware(t *testing.T) {
	mw1 := func(r *http.Request) (*http.Request, error) { return r, nil }
	mw2 := func(r *http.Request) (*http.Request, error) { return r, nil }

	c := NewClient("http://example.com", WithRequestMiddleware(mw1, mw2))
	assert.Len(t, c.requestMiddleware, 2)
}

func TestNewClient_WithResponseMiddleware(t *testing.T) {
	mw1 := func(r *http.Response) (*http.Response, error) { return r, nil }
	mw2 := func(r *http.Response) (*http.Response, error) { return r, nil }

	c := NewClient("http://example.com", WithResponseMiddleware(mw1, mw2))
	assert.Len(t, c.responseMiddleware, 2)
}

func TestNewClient_MultipleOptions(t *testing.T) {
	custom := &http.Client{}
	resolver := DefaultURLResolver{BaseURL: "http://custom.com"}
	reqMw := func(r *http.Request) (*http.Request, error) { return r, nil }
	resMw := func(r *http.Response) (*http.Response, error) { return r, nil }

	c := NewClient("http://example.com",
		WithHTTPClient(custom),
		WithURLResolver(resolver),
		WithRequestMiddleware(reqMw),
		WithResponseMiddleware(resMw),
	)

	assert.Equal(t, custom, c.httpClient)
	assert.Equal(t, resolver, c.resolver)
	assert.Len(t, c.requestMiddleware, 1)
	assert.Len(t, c.responseMiddleware, 1)
}

func TestNewClient_MiddlewareAppends(t *testing.T) {
	mw1 := func(r *http.Request) (*http.Request, error) { return r, nil }
	mw2 := func(r *http.Request) (*http.Request, error) { return r, nil }

	c := NewClient("http://example.com",
		WithRequestMiddleware(mw1),
		WithRequestMiddleware(mw2),
	)
	assert.Len(t, c.requestMiddleware, 2)
}

func TestNewClient_NilMiddlewareSlices(t *testing.T) {
	c := NewClient("http://example.com")
	assert.Nil(t, c.requestMiddleware)
	assert.Nil(t, c.responseMiddleware)
}

// Feature: jsonapi-http-client, Property 9: UnmarshalIncluded filters by type
// Validates: Requirements 18.2, 18.3, 18.4

// includedTestResource is a flexible test resource whose type can be set dynamically.
// This is needed for property testing UnmarshalIncluded with mixed resource types.
type includedTestResource struct {
	ID   string `jsonapi:"primary,included_test" json:"-"`
	Type string `json:"-"`
	Name string `jsonapi:"attr,name" json:"name"`
}

func (r includedTestResource) ResourceID() string   { return r.ID }
func (r includedTestResource) ResourceType() string { return r.Type }
func (r *includedTestResource) SetResourceID(id string) error {
	r.ID = id
	return nil
}

func TestUnmarshalIncluded_FiltersByType(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("only resources matching the requested type are unmarshaled", func(t *testing.T) {
		f := func(numTarget, numOther uint8) bool {
			// Clamp counts to avoid huge slices; ensure at least 0.
			targetCount := int(numTarget%20) + 1
			otherCount := int(numOther % 20)

			targetType := "target_type"
			otherType := "other_type"

			// Build included resources of mixed types.
			var included []*Resource
			for i := 0; i < targetCount; i++ {
				attrs, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("target-%d", i)})
				included = append(included, &Resource{
					Type:       targetType,
					ID:         fmt.Sprintf("t%d", i),
					Attributes: attrs,
				})
			}
			for i := 0; i < otherCount; i++ {
				attrs, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("other-%d", i)})
				included = append(included, &Resource{
					Type:       otherType,
					ID:         fmt.Sprintf("o%d", i),
					Attributes: attrs,
				})
			}

			resp := &Response{
				StatusCode: 200,
				document: &Document{
					Included: included,
				},
			}

			var results []includedTestResource
			err := resp.UnmarshalIncluded(targetType, &results)
			if err != nil {
				return false
			}

			// Must get exactly targetCount results.
			if len(results) != targetCount {
				return false
			}

			// Every result must have the target type's ID prefix and correct name.
			for i, r := range results {
				if r.ID != fmt.Sprintf("t%d", i) {
					return false
				}
				if r.Name != fmt.Sprintf("target-%d", i) {
					return false
				}
			}

			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("returns empty slice when no resources match the requested type", func(t *testing.T) {
		f := func(numResources uint8) bool {
			count := int(numResources%20) + 1
			nonMatchingType := "some_type"
			requestedType := "requested_type"

			var included []*Resource
			for i := 0; i < count; i++ {
				attrs, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("item-%d", i)})
				included = append(included, &Resource{
					Type:       nonMatchingType,
					ID:         fmt.Sprintf("s%d", i),
					Attributes: attrs,
				})
			}

			resp := &Response{
				StatusCode: 200,
				document: &Document{
					Included: included,
				},
			}

			var results []includedTestResource
			err := resp.UnmarshalIncluded(requestedType, &results)
			if err != nil {
				return false
			}

			// Must be an empty slice, not nil.
			return results != nil && len(results) == 0
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("filters correctly with randomly generated type names", func(t *testing.T) {
		f := func(targetType, otherType string, numTarget, numOther uint8) bool {
			// Skip if types are the same — can't test filtering with identical types.
			if targetType == otherType {
				return true
			}

			targetCount := int(numTarget%10) + 1
			otherCount := int(numOther % 10)

			var included []*Resource
			for i := 0; i < targetCount; i++ {
				attrs, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("t-%d", i)})
				included = append(included, &Resource{
					Type:       targetType,
					ID:         fmt.Sprintf("t%d", i),
					Attributes: attrs,
				})
			}
			for i := 0; i < otherCount; i++ {
				attrs, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("o-%d", i)})
				included = append(included, &Resource{
					Type:       otherType,
					ID:         fmt.Sprintf("o%d", i),
					Attributes: attrs,
				})
			}

			resp := &Response{
				StatusCode: 200,
				document: &Document{
					Included: included,
				},
			}

			var results []includedTestResource
			err := resp.UnmarshalIncluded(targetType, &results)
			if err != nil {
				return false
			}

			return len(results) == targetCount
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Feature: jsonapi-http-client, Property 8: Response document fields are accessible
// Validates: Requirements 17.1, 17.2, 17.3, 17.4, 17.5

// randomDocument holds randomly generated document fields for property testing.
// It implements quick.Generator so testing/quick can produce random instances.
type randomDocument struct {
	StatusCode int
	Links      map[string]Link
	Meta       map[string]interface{}
	Errors     []*Error
	Included   []*Resource
}

func (randomDocument) Generate(rand *rand.Rand, size int) reflect.Value {
	rd := randomDocument{}

	// Random status code in the HTTP range.
	rd.StatusCode = 100 + rand.Intn(500)

	// Generate random links.
	numLinks := rand.Intn(5)
	if numLinks > 0 {
		rd.Links = make(map[string]Link, numLinks)
		for i := 0; i < numLinks; i++ {
			key := fmt.Sprintf("link-%d", i)
			rd.Links[key] = Link{Href: fmt.Sprintf("http://example.com/%d", rand.Int())}
		}
	}

	// Generate random meta.
	numMeta := rand.Intn(5)
	if numMeta > 0 {
		rd.Meta = make(map[string]interface{}, numMeta)
		for i := 0; i < numMeta; i++ {
			key := fmt.Sprintf("meta-%d", i)
			rd.Meta[key] = fmt.Sprintf("value-%d", rand.Int())
		}
	}

	// Generate random errors.
	numErrors := rand.Intn(4)
	if numErrors > 0 {
		rd.Errors = make([]*Error, numErrors)
		for i := 0; i < numErrors; i++ {
			rd.Errors[i] = &Error{
				ID:     fmt.Sprintf("err-%d", i),
				Status: fmt.Sprintf("%d", 400+rand.Intn(100)),
				Title:  fmt.Sprintf("Error %d", i),
				Detail: fmt.Sprintf("Detail for error %d", i),
			}
		}
	}

	// Generate random included resources.
	numIncluded := rand.Intn(5)
	if numIncluded > 0 {
		rd.Included = make([]*Resource, numIncluded)
		for i := 0; i < numIncluded; i++ {
			rd.Included[i] = &Resource{
				Type: fmt.Sprintf("type-%d", i),
				ID:   fmt.Sprintf("id-%d", i),
			}
		}
	}

	return reflect.ValueOf(rd)
}

func TestResponse_DocumentFieldsAccessible(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("Links returns the document links", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			resp := &Response{
				StatusCode: rd.StatusCode,
				document: &Document{
					Links: rd.Links,
				},
			}
			got := resp.Links()
			if rd.Links == nil {
				return got == nil
			}
			if len(got) != len(rd.Links) {
				return false
			}
			for k, v := range rd.Links {
				if got[k].Href != v.Href {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Meta returns the document meta", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			resp := &Response{
				StatusCode: rd.StatusCode,
				document: &Document{
					Meta: rd.Meta,
				},
			}
			got := resp.Meta()
			if rd.Meta == nil {
				return got == nil
			}
			if len(got) != len(rd.Meta) {
				return false
			}
			for k, v := range rd.Meta {
				if got[k] != v {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Errors returns the document errors", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			resp := &Response{
				StatusCode: rd.StatusCode,
				document: &Document{
					Errors: rd.Errors,
				},
			}
			got := resp.Errors()
			if rd.Errors == nil {
				return got == nil
			}
			if len(got) != len(rd.Errors) {
				return false
			}
			for i, e := range rd.Errors {
				if got[i].ID != e.ID || got[i].Status != e.Status || got[i].Title != e.Title || got[i].Detail != e.Detail {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("HasErrors returns true when errors exist and false otherwise", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			resp := &Response{
				StatusCode: rd.StatusCode,
				document: &Document{
					Errors: rd.Errors,
				},
			}
			if len(rd.Errors) > 0 {
				return resp.HasErrors() == true
			}
			return resp.HasErrors() == false
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("StatusCode is accessible", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			resp := &Response{
				StatusCode: rd.StatusCode,
				document:   &Document{},
			}
			return resp.StatusCode == rd.StatusCode
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Document returns the document", func(t *testing.T) {
		f := func(rd randomDocument) bool {
			doc := &Document{
				Links:    rd.Links,
				Meta:     rd.Meta,
				Errors:   rd.Errors,
				Included: rd.Included,
			}
			resp := &Response{
				StatusCode: rd.StatusCode,
				document:   doc,
			}
			return resp.Document() == doc
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("all accessors return nil when document is nil", func(t *testing.T) {
		f := func(statusCode uint16) bool {
			resp := &Response{
				StatusCode: int(statusCode),
				document:   nil,
			}
			return resp.Links() == nil &&
				resp.Meta() == nil &&
				resp.Errors() == nil &&
				resp.HasErrors() == false &&
				resp.Document() == nil
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Task 5.1: Unit tests for doRequest helper
// Requirements: 1.4, 1.5, 15.1, 15.2, 15.3, 16.2, 16.4

func TestDoRequest_SetsAcceptHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoRequest_SetsContentTypeWhenBodyPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
		assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	body := []byte(`{"data":{"type":"test","attributes":{}}}`)
	resp, err := c.doRequest(context.Background(), http.MethodPost, srv.URL+"/test", body)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestDoRequest_NoContentTypeWhenBodyNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
		assert.Empty(t, r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoRequest_204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodDelete, srv.URL+"/test/1", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Nil(t, resp.Document())
}

func TestDoRequest_NonTwoXXWithJSONAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(Document{
			Errors: []*Error{
				{Status: "404", Title: "Not Found", Detail: "Resource not found"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/999", nil)
	assert.Error(t, err)
	var respErr *ResponseError
	assert.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
	assert.True(t, respErr.HasErrors())
	assert.Len(t, respErr.Errors(), 1)
	assert.Equal(t, "Not Found", respErr.Errors()[0].Title)
}

func TestDoRequest_NonTwoXXWithNonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	var respErr *ResponseError
	assert.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestDoRequest_ParsesSuccessfulResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		doc := Document{
			Data: &DocumentData{one: Resource{ID: "42", Type: "articles"}},
			Meta: map[string]interface{}{"total": float64(100)},
			Links: map[string]Link{
				"self": {Href: "http://example.com/articles/42"},
			},
		}
		json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/articles/42", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, resp.Document())
	assert.NotNil(t, resp.Links())
	assert.NotNil(t, resp.Meta())
}

func TestDoRequest_AppliesQueryParameters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "author,tags", r.URL.Query().Get("include"))
		assert.Equal(t, "title,content", r.URL.Query().Get("fields[articles]"))
		assert.Equal(t, "-created_at", r.URL.Query().Get("sort"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{many: []Resource{}, isMany: true}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/articles", nil,
		WithInclude("author", "tags"),
		WithFields("articles", "title", "content"),
		WithSort("-created_at"),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoRequest_AppliesRequestMiddleware(t *testing.T) {
	var middlewareOrder []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	mw1 := func(r *http.Request) (*http.Request, error) {
		middlewareOrder = append(middlewareOrder, "mw1")
		r.Header.Set("Authorization", "Bearer token123")
		return r, nil
	}
	mw2 := func(r *http.Request) (*http.Request, error) {
		middlewareOrder = append(middlewareOrder, "mw2")
		r.Header.Set("X-Custom", "custom-value")
		return r, nil
	}

	c := NewClient(srv.URL, WithRequestMiddleware(mw1, mw2))
	_, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{"mw1", "mw2"}, middlewareOrder)
}

func TestDoRequest_AppliesResponseMiddleware(t *testing.T) {
	var middlewareOrder []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	mw1 := func(r *http.Response) (*http.Response, error) {
		middlewareOrder = append(middlewareOrder, "rmw1")
		r.Header.Set("X-Processed-1", "true")
		return r, nil
	}
	mw2 := func(r *http.Response) (*http.Response, error) {
		middlewareOrder = append(middlewareOrder, "rmw2")
		r.Header.Set("X-Processed-2", "true")
		return r, nil
	}

	c := NewClient(srv.URL, WithResponseMiddleware(mw1, mw2))
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{"rmw1", "rmw2"}, middlewareOrder)
	assert.Equal(t, "true", resp.Header.Get("X-Processed-1"))
	assert.Equal(t, "true", resp.Header.Get("X-Processed-2"))
}

func TestDoRequest_RequestMiddlewareError(t *testing.T) {
	c := NewClient("http://example.com", WithRequestMiddleware(
		func(r *http.Request) (*http.Request, error) {
			return nil, fmt.Errorf("middleware failed")
		},
	))
	resp, err := c.doRequest(context.Background(), http.MethodGet, "http://example.com/test/1", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "middleware failed")
}

func TestDoRequest_ResponseMiddlewareError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithResponseMiddleware(
		func(r *http.Response) (*http.Response, error) {
			return nil, fmt.Errorf("response middleware failed")
		},
	))
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "response middleware failed")
}

func TestDoRequest_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	c := NewClient(srv.URL)
	resp, err := c.doRequest(ctx, http.MethodGet, srv.URL+"/test/1", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDoRequest_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test/1", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unmarshal response")
}

// Task 5.2: Unit tests for Fetch, List, Create, Update, Delete methods
// Requirements: 3.1-3.5, 4.1-4.4, 5.1-5.4, 6.1-6.5, 7.1-7.4

// clientTestResource is a test resource for Client CRUD method tests.
type clientTestResource struct {
	ID   string `jsonapi:"primary,articles" json:"-"`
	Name string `jsonapi:"attr,name" json:"name"`
}

func (r clientTestResource) ResourceID() string   { return r.ID }
func (r clientTestResource) ResourceType() string { return "articles" }
func (r *clientTestResource) SetResourceID(id string) error {
	r.ID = id
	return nil
}

func TestClient_Fetch(t *testing.T) {
	t.Run("sends GET to resource URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/articles/42", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Empty(t, r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			attrs, _ := json.Marshal(map[string]string{"name": "Test Article"})
			doc := Document{Data: &DocumentData{one: Resource{ID: "42", Type: "articles", Attributes: attrs}}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.Fetch(context.Background(), "articles", "42")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var article clientTestResource
		err = resp.Unmarshal(&article)
		assert.NoError(t, err)
		assert.Equal(t, "42", article.ID)
		assert.Equal(t, "Test Article", article.Name)
	})

	t.Run("passes query parameters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "author", r.URL.Query().Get("include"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "articles"}}})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.Fetch(context.Background(), "articles", "1", WithInclude("author"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "404", Title: "Not Found"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.Fetch(context.Background(), "articles", "999")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not json"))
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.Fetch(context.Background(), "articles", "1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal response")
	})
}

func TestClient_List(t *testing.T) {
	t.Run("sends GET to collection URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/articles", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			attrs1, _ := json.Marshal(map[string]string{"name": "Article 1"})
			attrs2, _ := json.Marshal(map[string]string{"name": "Article 2"})
			doc := Document{Data: &DocumentData{
				isMany: true,
				many: []Resource{
					{ID: "1", Type: "articles", Attributes: attrs1},
					{ID: "2", Type: "articles", Attributes: attrs2},
				},
			}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.List(context.Background(), "articles")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var articles []clientTestResource
		err = resp.Unmarshal(&articles)
		assert.NoError(t, err)
		assert.Len(t, articles, 2)
		assert.Equal(t, "Article 1", articles[0].Name)
		assert.Equal(t, "Article 2", articles[1].Name)
	})

	t.Run("passes query parameters for pagination and sorting", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1", r.URL.Query().Get("page[number]"))
			assert.Equal(t, "25", r.URL.Query().Get("page[size]"))
			assert.Equal(t, "-created_at", r.URL.Query().Get("sort"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{many: []Resource{}, isMany: true}})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.List(context.Background(), "articles",
			WithPageNumber(1, 25),
			WithSort("-created_at"),
		)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns pagination links", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{
				Data:  &DocumentData{many: []Resource{}, isMany: true},
				Links: map[string]Link{"next": {Href: "http://example.com/articles?page[number]=2"}},
			}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.List(context.Background(), "articles")
		assert.NoError(t, err)
		assert.NotNil(t, resp.Links())
		assert.Equal(t, "http://example.com/articles?page[number]=2", resp.Links()["next"].Href)
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "403", Title: "Forbidden"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.List(context.Background(), "articles")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusForbidden, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_Create(t *testing.T) {
	t.Run("sends POST to collection URL with marshaled body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/articles", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))

			// Verify the body is a valid JSON:API document with a "data" key.
			var raw map[string]json.RawMessage
			err := json.NewDecoder(r.Body).Decode(&raw)
			assert.NoError(t, err)
			_, hasData := raw["data"]
			assert.True(t, hasData, "request body should contain a 'data' key")

			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusCreated)
			attrs, _ := json.Marshal(map[string]string{"name": "New Article"})
			respDoc := Document{Data: &DocumentData{one: Resource{ID: "99", Type: "articles", Attributes: attrs}}}
			json.NewEncoder(w).Encode(respDoc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientTestResource{Name: "New Article"}
		resp, err := c.Create(context.Background(), resource)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var created clientTestResource
		err = resp.Unmarshal(&created)
		assert.NoError(t, err)
		assert.Equal(t, "99", created.ID)
		assert.Equal(t, "New Article", created.Name)
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "422", Title: "Validation Error", Detail: "Name is required"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientTestResource{}
		_, err := c.Create(context.Background(), resource)
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusUnprocessableEntity, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
		assert.Equal(t, "Validation Error", respErr.Errors()[0].Title)
	})
}

func TestClient_Update(t *testing.T) {
	t.Run("sends PATCH to resource URL with marshaled body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Equal(t, "/articles/42", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))

			// Verify the body is a valid JSON:API document with a "data" key.
			var raw map[string]json.RawMessage
			err := json.NewDecoder(r.Body).Decode(&raw)
			assert.NoError(t, err)
			_, hasData := raw["data"]
			assert.True(t, hasData, "request body should contain a 'data' key")

			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			attrs, _ := json.Marshal(map[string]string{"name": "Updated Article"})
			respDoc := Document{Data: &DocumentData{one: Resource{ID: "42", Type: "articles", Attributes: attrs}}}
			json.NewEncoder(w).Encode(respDoc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientTestResource{ID: "42", Name: "Updated Article"}
		resp, err := c.Update(context.Background(), resource)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var updated clientTestResource
		err = resp.Unmarshal(&updated)
		assert.NoError(t, err)
		assert.Equal(t, "42", updated.ID)
		assert.Equal(t, "Updated Article", updated.Name)
	})

	t.Run("handles 204 No Content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Equal(t, "/articles/42", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientTestResource{ID: "42", Name: "Updated Article"}
		resp, err := c.Update(context.Background(), resource)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "409", Title: "Conflict"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientTestResource{ID: "42", Name: "Updated Article"}
		_, err := c.Update(context.Background(), resource)
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusConflict, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_Delete(t *testing.T) {
	t.Run("sends DELETE to resource URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/articles/42", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Empty(t, r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.Delete(context.Background(), "articles", "42")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("handles 200 OK with body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{
				Meta: map[string]interface{}{"deleted": true},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.Delete(context.Background(), "articles", "42")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, resp.Meta())
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "404", Title: "Not Found"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.Delete(context.Background(), "articles", "999")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})

	t.Run("context cancellation returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		c := NewClient(srv.URL)
		_, err := c.Delete(ctx, "articles", "42")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// Feature: jsonapi-http-client, Property 2: Client sends correct HTTP method to resolver URL
// Validates: Requirements 3.1, 4.1, 5.1, 6.1, 7.1, 8.1, 9.1, 10.1, 11.1, 12.1

// clientRefTestResource implements RelationshipMarshaler for property testing
// relationship operations (UpdateRef, AddRef, RemoveRef).
type clientRefTestResource struct {
	id      string
	typ     string
	refName string
	refType RelationType
}

func (r clientRefTestResource) ResourceID() string   { return r.id }
func (r clientRefTestResource) ResourceType() string { return r.typ }
func (r clientRefTestResource) Relationships() map[string]RelationType {
	return map[string]RelationType{r.refName: r.refType}
}
func (r clientRefTestResource) MarshalRef(name string) []ResourceIdentifier {
	if name == r.refName {
		return []ResourceIdentifier{clientRefTarget{id: "ref1", typ: r.typ + "_ref"}}
	}
	return nil
}

// clientRefTarget is a minimal ResourceIdentifier used as a relationship target.
type clientRefTarget struct {
	id  string
	typ string
}

func (r clientRefTarget) ResourceID() string   { return r.id }
func (r clientRefTarget) ResourceType() string { return r.typ }

// alphanumInput generates a simple alphanumeric string from a uint64 seed.
// This avoids URL encoding issues that would complicate path matching.
func alphanumInput(seed uint64) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	if seed == 0 {
		return "a"
	}
	var result []byte
	v := seed
	for v > 0 {
		result = append(result, chars[v%uint64(len(chars))])
		v /= uint64(len(chars))
	}
	return string(result)
}

func TestClient_CorrectHTTPMethodAndURL(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("Fetch sends GET to ResolveResourceURL", func(t *testing.T) {
		f := func(typSeed, idSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Fetch(context.Background(), typ, id)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id
			return gotMethod == http.MethodGet && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("List sends GET to ResolveCollectionURL", func(t *testing.T) {
		f := func(typSeed uint64) bool {
			typ := alphanumInput(typSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{many: []Resource{}, isMany: true}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.List(context.Background(), typ)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ
			return gotMethod == http.MethodGet && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Create sends POST to ResolveCollectionURL", func(t *testing.T) {
		f := func(typSeed, idSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientTestResource{ID: id, Name: "test"}
			_, err := c.Create(context.Background(), resource)
			if err != nil {
				return false
			}

			// Create uses resource.ResourceType() which is "articles" for clientTestResource
			expectedPath := "/articles"
			return gotMethod == http.MethodPost && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Update sends PATCH to ResolveResourceURL", func(t *testing.T) {
		f := func(idSeed uint64) bool {
			id := alphanumInput(idSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: "articles"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientTestResource{ID: id, Name: "updated"}
			_, err := c.Update(context.Background(), resource)
			if err != nil {
				return false
			}

			// Update uses resource.ResourceType() ("articles") and resource.ResourceID()
			expectedPath := "/articles/" + id
			return gotMethod == http.MethodPatch && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Delete sends DELETE to ResolveResourceURL", func(t *testing.T) {
		f := func(typSeed, idSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Delete(context.Background(), typ, id)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id
			return gotMethod == http.MethodDelete && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("FetchRef sends GET to ResolveRelationshipURL", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "ref1", Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.FetchRef(context.Background(), typ, id, rel)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id + "/relationships/" + rel
			return gotMethod == http.MethodGet && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("UpdateRef sends PATCH to ResolveRelationshipURL", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToOne}
			_, err := c.UpdateRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id + "/relationships/" + rel
			return gotMethod == http.MethodPatch && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("AddRef sends POST to ResolveRelationshipURL", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			_, err := c.AddRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id + "/relationships/" + rel
			return gotMethod == http.MethodPost && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("RemoveRef sends DELETE to ResolveRelationshipURL", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			_, err := c.RemoveRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id + "/relationships/" + rel
			return gotMethod == http.MethodDelete && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("FetchRelated sends GET to ResolveRelatedURL", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.FetchRelated(context.Background(), typ, id, rel)
			if err != nil {
				return false
			}

			expectedPath := "/" + typ + "/" + id + "/" + rel
			return gotMethod == http.MethodGet && gotPath == expectedPath
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Feature: jsonapi-http-client, Property 3: JSON:API headers on all requests
// Validates: Requirements 1.4, 1.5

func TestClient_JSONAPIHeadersOnAllRequests(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	// Operations without a body: Accept header present, Content-Type absent.

	t.Run("Fetch sets Accept but not Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Fetch(context.Background(), typ, id)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == ""
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("List sets Accept but not Content-Type", func(t *testing.T) {
		f := func(typSeed uint64) bool {
			typ := alphanumInput(typSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{many: []Resource{}, isMany: true}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.List(context.Background(), typ)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == ""
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Delete sets Accept but not Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Delete(context.Background(), typ, id)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == ""
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("FetchRef sets Accept but not Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "ref1", Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.FetchRef(context.Background(), typ, id, rel)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == ""
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("FetchRelated sets Accept but not Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: typ}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.FetchRelated(context.Background(), typ, id, rel)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == ""
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	// Operations with a body: both Accept and Content-Type present.

	t.Run("Create sets both Accept and Content-Type", func(t *testing.T) {
		f := func(idSeed uint64) bool {
			id := alphanumInput(idSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: "articles"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientTestResource{ID: id, Name: "test"}
			_, err := c.Create(context.Background(), resource)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == "application/vnd.api+json"
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Update sets both Accept and Content-Type", func(t *testing.T) {
		f := func(idSeed uint64) bool {
			id := alphanumInput(idSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: "articles"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientTestResource{ID: id, Name: "updated"}
			_, err := c.Update(context.Background(), resource)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == "application/vnd.api+json"
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("UpdateRef sets both Accept and Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToOne}
			_, err := c.UpdateRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == "application/vnd.api+json"
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("AddRef sets both Accept and Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			_, err := c.AddRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == "application/vnd.api+json"
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("RemoveRef sets both Accept and Content-Type", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotAccept, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept")
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			_, err := c.RemoveRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return gotAccept == "application/vnd.api+json" && gotContentType == "application/vnd.api+json"
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Feature: jsonapi-http-client, Property 4: Request bodies are valid JSON:API documents
// Validates: Requirements 5.2, 6.2, 9.2, 10.2, 11.2

func TestClient_RequestBodiesAreValidJSONAPIDocuments(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("Create body matches Marshal output", func(t *testing.T) {
		f := func(idSeed, nameSeed uint64) bool {
			id := alphanumInput(idSeed)
			name := alphanumInput(nameSeed)

			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				gotBody = body
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: "articles"}}})
			}))
			defer srv.Close()

			resource := clientTestResource{ID: id, Name: name}
			expected, err := Marshal(resource)
			if err != nil {
				return false
			}

			c := NewClient(srv.URL)
			_, err = c.Create(context.Background(), resource)
			if err != nil {
				return false
			}

			return bytes.Equal(gotBody, expected)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("Update body matches Marshal output", func(t *testing.T) {
		f := func(idSeed, nameSeed uint64) bool {
			id := alphanumInput(idSeed)
			name := alphanumInput(nameSeed)

			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				gotBody = body
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: id, Type: "articles"}}})
			}))
			defer srv.Close()

			resource := clientTestResource{ID: id, Name: name}
			expected, err := Marshal(resource)
			if err != nil {
				return false
			}

			c := NewClient(srv.URL)
			_, err = c.Update(context.Background(), resource)
			if err != nil {
				return false
			}

			return bytes.Equal(gotBody, expected)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("UpdateRef body matches MarshalRef output", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				gotBody = body
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToOne}
			expected, err := MarshalRef(resource, rel)
			if err != nil {
				return false
			}

			c := NewClient(srv.URL)
			_, err = c.UpdateRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return bytes.Equal(gotBody, expected)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("AddRef body matches MarshalRef output", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				gotBody = body
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			expected, err := MarshalRef(resource, rel)
			if err != nil {
				return false
			}

			c := NewClient(srv.URL)
			_, err = c.AddRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return bytes.Equal(gotBody, expected)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("RemoveRef body matches MarshalRef output", func(t *testing.T) {
		f := func(typSeed, idSeed, relSeed uint64) bool {
			typ := alphanumInput(typSeed)
			id := alphanumInput(idSeed)
			rel := alphanumInput(relSeed)

			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return
				}
				gotBody = body
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			resource := clientRefTestResource{id: id, typ: typ, refName: rel, refType: RelationToMany}
			expected, err := MarshalRef(resource, rel)
			if err != nil {
				return false
			}

			c := NewClient(srv.URL)
			_, err = c.RemoveRef(context.Background(), resource, rel)
			if err != nil {
				return false
			}

			return bytes.Equal(gotBody, expected)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Feature: jsonapi-http-client, Property 5: Non-2xx responses produce accessible JSON:API errors
// Validates: Requirements 3.4, 4.4, 5.4, 6.5, 7.4, 8.3, 9.4, 10.4, 11.4, 12.4

// randomErrorResponse holds randomly generated error response data for property testing.
// It implements quick.Generator so testing/quick can produce random instances.
type randomErrorResponse struct {
	StatusCode int
	Errors     []Error
}

func (randomErrorResponse) Generate(rand *rand.Rand, size int) reflect.Value {
	r := randomErrorResponse{}

	// Generate a random 4xx or 5xx status code (400-599).
	r.StatusCode = 400 + rand.Intn(200)

	// Generate 1-5 random error objects (at least one so the response has errors).
	numErrors := 1 + rand.Intn(5)
	r.Errors = make([]Error, numErrors)
	for i := 0; i < numErrors; i++ {
		r.Errors[i] = Error{
			ID:     fmt.Sprintf("err-%d-%d", i, rand.Intn(10000)),
			Status: fmt.Sprintf("%d", r.StatusCode),
			Title:  fmt.Sprintf("Error Title %d", rand.Intn(10000)),
			Detail: fmt.Sprintf("Error detail message %d", rand.Intn(10000)),
		}
	}

	return reflect.ValueOf(r)
}

func TestClient_NonTwoXXErrorResponsesAccessible(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("HasErrors returns true and Errors returns correct error objects", func(t *testing.T) {
		f := func(re randomErrorResponse) bool {
			// Build the error pointers for the document.
			errPtrs := make([]*Error, len(re.Errors))
			for i := range re.Errors {
				errPtrs[i] = &re.Errors[i]
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(re.StatusCode)
				doc := Document{Errors: errPtrs}
				json.NewEncoder(w).Encode(doc)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Fetch(context.Background(), "resources", "1")
			if err == nil {
				// Non-2xx must always return an error now.
				return false
			}

			var respErr *ResponseError
			if !errors.As(err, &respErr) {
				return false
			}

			// Verify HasErrors returns true.
			if !respErr.HasErrors() {
				return false
			}

			// Verify the status code matches.
			if respErr.StatusCode != re.StatusCode {
				return false
			}

			// Verify the correct number of errors.
			gotErrors := respErr.Errors()
			if len(gotErrors) != len(re.Errors) {
				return false
			}

			// Verify each error's fields match what was sent.
			for i, expected := range re.Errors {
				got := gotErrors[i]
				if got.ID != expected.ID {
					return false
				}
				if got.Status != expected.Status {
					return false
				}
				if got.Title != expected.Title {
					return false
				}
				if got.Detail != expected.Detail {
					return false
				}
			}

			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("error fields are individually accessible", func(t *testing.T) {
		f := func(re randomErrorResponse) bool {
			errPtrs := make([]*Error, len(re.Errors))
			for i := range re.Errors {
				errPtrs[i] = &re.Errors[i]
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(re.StatusCode)
				doc := Document{Errors: errPtrs}
				json.NewEncoder(w).Encode(doc)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Fetch(context.Background(), "resources", "1")
			if err == nil {
				return false
			}

			var respErr *ResponseError
			if !errors.As(err, &respErr) {
				return false
			}

			// Verify each error can be accessed by index and all fields are correct.
			for i, expected := range re.Errors {
				got := respErr.Errors()[i]
				if got.ID != expected.ID || got.Status != expected.Status ||
					got.Title != expected.Title || got.Detail != expected.Detail {
					return false
				}
			}

			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("various 4xx and 5xx status codes all produce accessible errors", func(t *testing.T) {
		f := func(re randomErrorResponse) bool {
			errPtrs := make([]*Error, len(re.Errors))
			for i := range re.Errors {
				errPtrs[i] = &re.Errors[i]
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(re.StatusCode)
				doc := Document{Errors: errPtrs}
				json.NewEncoder(w).Encode(doc)
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Fetch(context.Background(), "items", "42")
			if err == nil {
				return false
			}

			var respErr *ResponseError
			if !errors.As(err, &respErr) {
				return false
			}

			// The status code must be in the 4xx/5xx range.
			if respErr.StatusCode < 400 || respErr.StatusCode > 599 {
				return false
			}

			// HasErrors must be true and error count must match.
			return respErr.HasErrors() && len(respErr.Errors()) == len(re.Errors)
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Task 5.8: Unit tests for Client HTTP methods (FetchRef, UpdateRef, AddRef, RemoveRef, FetchRelated)
// Requirements: 8.1–8.3, 9.1–9.4, 10.1–10.4, 11.1–11.4, 12.1–12.4, 15.1–15.3

func TestClient_FetchRef(t *testing.T) {
	t.Run("sends GET to relationship URL and returns relationship data", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/articles/42/relationships/author", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Empty(t, r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{Data: &DocumentData{one: Resource{ID: "10", Type: "people"}}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.FetchRef(context.Background(), "articles", "42", "author")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, resp.Document())
	})

	t.Run("passes query parameters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "name", r.URL.Query().Get("fields[people]"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{Data: &DocumentData{one: Resource{ID: "10", Type: "people"}}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.FetchRef(context.Background(), "articles", "42", "author", WithFields("people", "name"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "404", Title: "Not Found"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.FetchRef(context.Background(), "articles", "999", "author")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_UpdateRef(t *testing.T) {
	t.Run("sends PATCH to relationship URL with marshaled body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Equal(t, "/posts/1/relationships/author", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))

			// Verify the body is valid JSON with a "data" key.
			var raw map[string]json.RawMessage
			err := json.NewDecoder(r.Body).Decode(&raw)
			assert.NoError(t, err)
			_, hasData := raw["data"]
			assert.True(t, hasData, "request body should contain a 'data' key")

			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{Data: &DocumentData{one: Resource{ID: "ref1", Type: "posts_ref"}}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "1", typ: "posts", refName: "author", refType: RelationToOne}
		resp, err := c.UpdateRef(context.Background(), resource, "author")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("handles 204 No Content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Equal(t, "/posts/1/relationships/author", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "1", typ: "posts", refName: "author", refType: RelationToOne}
		resp, err := c.UpdateRef(context.Background(), resource, "author")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "403", Title: "Forbidden"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "1", typ: "posts", refName: "author", refType: RelationToOne}
		_, err := c.UpdateRef(context.Background(), resource, "author")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusForbidden, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_AddRef(t *testing.T) {
	t.Run("sends POST to relationship URL with marshaled body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/articles/5/relationships/tags", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))

			// Verify the body is valid JSON with a "data" key.
			var raw map[string]json.RawMessage
			err := json.NewDecoder(r.Body).Decode(&raw)
			assert.NoError(t, err)
			_, hasData := raw["data"]
			assert.True(t, hasData, "request body should contain a 'data' key")

			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "ref1", Type: "articles_ref"}},
			}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "5", typ: "articles", refName: "tags", refType: RelationToMany}
		resp, err := c.AddRef(context.Background(), resource, "tags")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("handles 204 No Content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/articles/5/relationships/tags", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "5", typ: "articles", refName: "tags", refType: RelationToMany}
		resp, err := c.AddRef(context.Background(), resource, "tags")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "409", Title: "Conflict"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "5", typ: "articles", refName: "tags", refType: RelationToMany}
		_, err := c.AddRef(context.Background(), resource, "tags")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusConflict, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_RemoveRef(t *testing.T) {
	t.Run("sends DELETE to relationship URL with marshaled body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/articles/5/relationships/tags", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))

			// Verify the body is valid JSON with a "data" key.
			var raw map[string]json.RawMessage
			err := json.NewDecoder(r.Body).Decode(&raw)
			assert.NoError(t, err)
			_, hasData := raw["data"]
			assert.True(t, hasData, "request body should contain a 'data' key")

			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "5", typ: "articles", refName: "tags", refType: RelationToMany}
		resp, err := c.RemoveRef(context.Background(), resource, "tags")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("handles 204 No Content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/articles/7/relationships/comments", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "7", typ: "articles", refName: "comments", refType: RelationToMany}
		resp, err := c.RemoveRef(context.Background(), resource, "comments")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.Document())
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "404", Title: "Not Found"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resource := clientRefTestResource{id: "5", typ: "articles", refName: "tags", refType: RelationToMany}
		_, err := c.RemoveRef(context.Background(), resource, "tags")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

func TestClient_FetchRelated(t *testing.T) {
	t.Run("sends GET to related URL and returns single resource", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/articles/42/author", r.URL.Path)
			assert.Equal(t, "application/vnd.api+json", r.Header.Get("Accept"))
			assert.Empty(t, r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			attrs, _ := json.Marshal(map[string]string{"name": "Jane Doe"})
			doc := Document{Data: &DocumentData{one: Resource{ID: "10", Type: "people", Attributes: attrs}}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.FetchRelated(context.Background(), "articles", "42", "author")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, resp.Document())
	})

	t.Run("sends GET to related URL and returns collection", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/articles/42/tags", r.URL.Path)
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			attrs1, _ := json.Marshal(map[string]string{"name": "go"})
			attrs2, _ := json.Marshal(map[string]string{"name": "jsonapi"})
			doc := Document{Data: &DocumentData{
				isMany: true,
				many: []Resource{
					{ID: "1", Type: "tags", Attributes: attrs1},
					{ID: "2", Type: "tags", Attributes: attrs2},
				},
			}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.FetchRelated(context.Background(), "articles", "42", "tags")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, resp.Document())
	})

	t.Run("passes query parameters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "name", r.URL.Query().Get("fields[tags]"))
			assert.Equal(t, "name", r.URL.Query().Get("sort"))
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusOK)
			doc := Document{Data: &DocumentData{many: []Resource{}, isMany: true}}
			json.NewEncoder(w).Encode(doc)
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		resp, err := c.FetchRelated(context.Background(), "articles", "42", "tags",
			WithFields("tags", "name"),
			WithSort("name"),
		)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns error response on non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{
				Errors: []*Error{{Status: "404", Title: "Not Found"}},
			})
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		_, err := c.FetchRelated(context.Background(), "articles", "999", "author")
		assert.Error(t, err)
		var respErr *ResponseError
		assert.True(t, errors.As(err, &respErr))
		assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
		assert.True(t, respErr.HasErrors())
	})
}

// Task 5.8: Context deadline exceeded test
// Requirements: 15.1, 15.2, 15.3

func TestClient_ContextDeadlineExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server that never responds in time.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	c := NewClient(srv.URL)
	resp, err := c.Fetch(ctx, "articles", "1")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// Feature: jsonapi-http-client, Property 7: Middleware is applied in registration order
// Validates: Requirements 16.2, 16.4

// middlewareOrderInput holds randomly generated middleware counts for property testing.
type middlewareOrderInput struct {
	N uint8 // number of request middleware (1-10)
	M uint8 // number of response middleware (1-10)
}

func (middlewareOrderInput) Generate(rand *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(middlewareOrderInput{
		N: uint8(rand.Intn(10)) + 1,
		M: uint8(rand.Intn(10)) + 1,
	})
}

func TestMiddleware_AppliedInRegistrationOrder(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("request middleware executes in registration order", func(t *testing.T) {
		f := func(input middlewareOrderInput) bool {
			n := int(input.N)

			var reqOrder []int

			// Build N request middleware, each appending its index.
			var reqMiddleware []RequestMiddleware
			for i := 0; i < n; i++ {
				idx := i
				reqMiddleware = append(reqMiddleware, func(r *http.Request) (*http.Request, error) {
					reqOrder = append(reqOrder, idx)
					return r, nil
				})
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL, WithRequestMiddleware(reqMiddleware...))
			reqOrder = nil // reset before request

			_, err := c.Fetch(context.Background(), "test", "1")
			if err != nil {
				return false
			}

			// Verify request middleware ran in order [0, 1, 2, ..., N-1].
			if len(reqOrder) != n {
				return false
			}
			for i := 0; i < n; i++ {
				if reqOrder[i] != i {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("response middleware executes in registration order", func(t *testing.T) {
		f := func(input middlewareOrderInput) bool {
			m := int(input.M)

			var respOrder []int

			// Build M response middleware, each appending its index.
			var respMiddleware []ResponseMiddleware
			for i := 0; i < m; i++ {
				idx := i
				respMiddleware = append(respMiddleware, func(r *http.Response) (*http.Response, error) {
					respOrder = append(respOrder, idx)
					return r, nil
				})
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL, WithResponseMiddleware(respMiddleware...))
			respOrder = nil // reset before request

			_, err := c.Fetch(context.Background(), "test", "1")
			if err != nil {
				return false
			}

			// Verify response middleware ran in order [0, 1, 2, ..., M-1].
			if len(respOrder) != m {
				return false
			}
			for i := 0; i < m; i++ {
				if respOrder[i] != i {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("both request and response middleware execute in registration order", func(t *testing.T) {
		f := func(input middlewareOrderInput) bool {
			n := int(input.N)
			m := int(input.M)

			var reqOrder []int
			var respOrder []int

			// Build N request middleware.
			var reqMiddleware []RequestMiddleware
			for i := 0; i < n; i++ {
				idx := i
				reqMiddleware = append(reqMiddleware, func(r *http.Request) (*http.Request, error) {
					reqOrder = append(reqOrder, idx)
					return r, nil
				})
			}

			// Build M response middleware.
			var respMiddleware []ResponseMiddleware
			for i := 0; i < m; i++ {
				idx := i
				respMiddleware = append(respMiddleware, func(r *http.Response) (*http.Response, error) {
					respOrder = append(respOrder, idx)
					return r, nil
				})
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: Resource{ID: "1", Type: "test"}}})
			}))
			defer srv.Close()

			c := NewClient(srv.URL,
				WithRequestMiddleware(reqMiddleware...),
				WithResponseMiddleware(respMiddleware...),
			)
			reqOrder = nil
			respOrder = nil

			_, err := c.Fetch(context.Background(), "test", "1")
			if err != nil {
				return false
			}

			// Verify request middleware order.
			if len(reqOrder) != n {
				return false
			}
			for i := 0; i < n; i++ {
				if reqOrder[i] != i {
					return false
				}
			}

			// Verify response middleware order.
			if len(respOrder) != m {
				return false
			}
			for i := 0; i < m; i++ {
				if respOrder[i] != i {
					return false
				}
			}

			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Task 7.3: Unit tests for PageIterator
// Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8

func TestPageIterator_MultiPageIteration(t *testing.T) {
	// httptest.Server serving 3 pages: page1 → page2 → page3 (no next).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/articles/page2":
			attrs1, _ := json.Marshal(map[string]string{"name": "Article 3"})
			attrs2, _ := json.Marshal(map[string]string{"name": "Article 4"})
			doc := Document{
				Data: &DocumentData{
					isMany: true,
					many: []Resource{
						{ID: "3", Type: "articles", Attributes: attrs1},
						{ID: "4", Type: "articles", Attributes: attrs2},
					},
				},
				Links: map[string]Link{
					"next": {Href: ""}, // will be replaced below
				},
			}
			// Set the next link to point to page3 on this server.
			doc.Links["next"] = Link{Href: "http://" + r.Host + "/articles/page3"}
			json.NewEncoder(w).Encode(doc)

		case "/articles/page3":
			attrs1, _ := json.Marshal(map[string]string{"name": "Article 5"})
			doc := Document{
				Data: &DocumentData{
					isMany: true,
					many: []Resource{
						{ID: "5", Type: "articles", Attributes: attrs1},
					},
				},
				// No "next" link — this is the last page.
			}
			json.NewEncoder(w).Encode(doc)

		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Build the initial response (page 1) with a next link pointing to page2.
	attrs1, _ := json.Marshal(map[string]string{"name": "Article 1"})
	attrs2, _ := json.Marshal(map[string]string{"name": "Article 2"})
	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many: []Resource{
					{ID: "1", Type: "articles", Attributes: attrs1},
					{ID: "2", Type: "articles", Attributes: attrs2},
				},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles/page2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	// Unmarshal page 1 items from the initial response.
	var page1 []clientTestResource
	err := iter.Items(&page1)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, "Article 1", page1[0].Name)
	assert.Equal(t, "Article 2", page1[1].Name)

	// Iterate through remaining pages.
	var allItems []clientTestResource
	allItems = append(allItems, page1...)

	pageCount := 1
	for iter.Next(context.Background()) {
		pageCount++
		var items []clientTestResource
		err := iter.Items(&items)
		assert.NoError(t, err)
		allItems = append(allItems, items...)
	}
	assert.NoError(t, iter.Err())
	assert.Equal(t, 3, pageCount) // page1 + page2 + page3
	assert.Len(t, allItems, 5)
	assert.Equal(t, "Article 5", allItems[4].Name)
}

func TestPageIterator_TerminatesWhenNoNextLink(t *testing.T) {
	// Response with no "next" link — Next() should return false immediately.
	resp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			// No links at all.
		},
	}

	c := NewClient("http://example.com")
	iter := c.Pages(resp)

	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

func TestPageIterator_TerminatesWhenNextLinkEmpty(t *testing.T) {
	// Response with an empty "next" link href — Next() should return false.
	resp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			Links: map[string]Link{
				"next": {Href: ""},
			},
		},
	}

	c := NewClient("http://example.com")
	iter := c.Pages(resp)

	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

func TestPageIterator_TerminatesWhenLinksNil(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			Links: nil,
		},
	}

	c := NewClient("http://example.com")
	iter := c.Pages(resp)

	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

func TestPageIterator_ErrorPropagationOnNonTwoXX(t *testing.T) {
	// Server returns an error on the second page.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Document{
			Errors: []*Error{{Status: "500", Title: "Internal Server Error", Detail: "something went wrong"}},
		})
	}))
	defer srv.Close()

	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles/page2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	// Next should return false because the page fetch returned a non-2xx with JSON:API errors.
	assert.False(t, iter.Next(context.Background()))
	assert.Error(t, iter.Err())
	var respErr *ResponseError
	assert.True(t, errors.As(iter.Err(), &respErr))
	assert.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
	assert.True(t, respErr.HasErrors())
	assert.Contains(t, iter.Err().Error(), "Internal Server Error")
}

func TestPageIterator_ErrorPropagationOnNonJSONResponse(t *testing.T) {
	// Server returns a non-2xx with a non-JSON body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway"))
	}))
	defer srv.Close()

	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles/page2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	// Next should return false because doRequest returns an error for non-JSON non-2xx.
	assert.False(t, iter.Next(context.Background()))
	assert.Error(t, iter.Err())
	var respErr *ResponseError
	assert.True(t, errors.As(iter.Err(), &respErr))
	assert.Equal(t, http.StatusBadGateway, respErr.StatusCode)
	assert.Contains(t, iter.Err().Error(), "502")
}

func TestPageIterator_ContextCancellation(t *testing.T) {
	// Server that blocks until context is done.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles/page2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	assert.False(t, iter.Next(ctx))
	assert.Error(t, iter.Err())
	assert.ErrorIs(t, iter.Err(), context.Canceled)
}

func TestPageIterator_CollectionResponse(t *testing.T) {
	// Verify PageIterator works for collection responses (GET /{type}).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		attrs, _ := json.Marshal(map[string]string{"name": "Page 2 Article"})
		doc := Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "3", Type: "articles", Attributes: attrs}},
			},
			// No next link — last page.
		}
		json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	// Simulate a List response (collection) as the initial page.
	attrs, _ := json.Marshal(map[string]string{"name": "Page 1 Article"})
	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many: []Resource{
					{ID: "1", Type: "articles", Attributes: attrs},
					{ID: "2", Type: "articles", Attributes: attrs},
				},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles?page[number]=2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	// Unmarshal page 1.
	var page1 []clientTestResource
	err := iter.Items(&page1)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	// Fetch page 2.
	assert.True(t, iter.Next(context.Background()))
	var page2 []clientTestResource
	err = iter.Items(&page2)
	assert.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.Equal(t, "Page 2 Article", page2[0].Name)

	// No more pages.
	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

func TestPageIterator_RelatedResourceResponse(t *testing.T) {
	// Verify PageIterator works for related-resource responses (GET /{type}/{id}/{related}).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		attrs, _ := json.Marshal(map[string]string{"name": "Comment 3"})
		doc := Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "3", Type: "comments", Attributes: attrs}},
			},
			// No next link — last page.
		}
		json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	// Simulate a FetchRelated response as the initial page.
	attrs1, _ := json.Marshal(map[string]string{"name": "Comment 1"})
	attrs2, _ := json.Marshal(map[string]string{"name": "Comment 2"})
	initialResp := &Response{
		StatusCode: 200,
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many: []Resource{
					{ID: "1", Type: "comments", Attributes: attrs1},
					{ID: "2", Type: "comments", Attributes: attrs2},
				},
			},
			Links: map[string]Link{
				"next": {Href: srv.URL + "/articles/42/comments?page[number]=2"},
			},
		},
	}

	c := NewClient(srv.URL)
	iter := c.Pages(initialResp)

	// Unmarshal page 1 — use includedTestResource since comments aren't "articles" type.
	// Actually, let's just verify the iteration works by checking Items doesn't error
	// and the response is accessible.
	assert.NotNil(t, iter.Response())
	assert.Equal(t, 200, iter.Response().StatusCode)

	// Fetch page 2.
	assert.True(t, iter.Next(context.Background()))
	assert.NotNil(t, iter.Response())
	assert.NotNil(t, iter.Response().Document())

	// No more pages.
	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

func TestPageIterator_ResponseAccessor(t *testing.T) {
	// Verify Response() returns the current page's response.
	resp := &Response{
		StatusCode: 200,
		Header:     http.Header{"X-Custom": []string{"value"}},
		document: &Document{
			Data: &DocumentData{
				isMany: true,
				many:   []Resource{{ID: "1", Type: "articles"}},
			},
		},
	}

	c := NewClient("http://example.com")
	iter := c.Pages(resp)

	assert.Equal(t, resp, iter.Response())
	assert.Equal(t, 200, iter.Response().StatusCode)
}

func TestPageIterator_NilResponse(t *testing.T) {
	c := NewClient("http://example.com")
	iter := c.Pages(nil)

	assert.False(t, iter.Next(context.Background()))
	assert.NoError(t, iter.Err())
}

// Feature: jsonapi-http-client, Property 10: Resource marshal/unmarshal round-trip
// Validates: Requirements 19.1

// roundTripResource is a test resource for property testing the marshal/unmarshal round-trip.
// It implements both ResourceIdentifier and ResourceUnmarshaler with a configurable type.
type roundTripResource struct {
	ResID   string `json:"-"`
	ResType string `json:"-"`
	Name    string `json:"name"`
}

func (r roundTripResource) ResourceID() string   { return r.ResID }
func (r roundTripResource) ResourceType() string { return r.ResType }
func (r *roundTripResource) SetResourceID(id string) error {
	r.ResID = id
	return nil
}

// Generate implements quick.Generator for roundTripResource.
// It produces random resources with non-empty type, ID, and name attributes.
func (roundTripResource) Generate(rand *rand.Rand, size int) reflect.Value {
	r := roundTripResource{
		ResID:   alphanumInput(uint64(rand.Intn(100000)) + 1),
		ResType: alphanumInput(uint64(rand.Intn(100000)) + 1),
		Name:    alphanumInput(uint64(rand.Intn(100000)) + 1),
	}
	return reflect.ValueOf(r)
}

func TestResourceMarshalUnmarshalRoundTrip(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("marshal then unmarshal preserves type, ID, and attributes", func(t *testing.T) {
		f := func(original roundTripResource) bool {
			// Marshal the resource to JSON:API bytes.
			data, err := Marshal(original)
			if err != nil {
				return false
			}

			// Unmarshal the bytes into a Document.
			doc := &Document{}
			if err := json.Unmarshal(data, doc); err != nil {
				return false
			}

			// Use Document.UnmarshalData to get the resource back.
			var result roundTripResource
			result.ResType = original.ResType // Set the type so unmarshal can match.
			if err := doc.UnmarshalData(&result); err != nil {
				return false
			}

			// Verify the round-tripped resource has the same type, ID, and attributes.
			return result.ResID == original.ResID &&
				result.ResourceType() == original.ResourceType() &&
				result.Name == original.Name
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Feature: jsonapi-http-client, Property 11: Relationship marshal/unmarshal round-trip
// Validates: Requirements 19.2

// roundTripRefResource implements both RelationshipMarshaler and RelationshipUnmarshaler
// for property testing the relationship marshal/unmarshal round-trip.
type roundTripRefResource struct {
	id      string
	typ     string
	refName string
	refType RelationType
	// For to-one relationships.
	toOneID   string
	toOneType string
	// For to-many relationships.
	toManyRefs []Ref
}

func (r roundTripRefResource) ResourceID() string   { return r.id }
func (r roundTripRefResource) ResourceType() string { return r.typ }
func (r roundTripRefResource) Relationships() map[string]RelationType {
	return map[string]RelationType{r.refName: r.refType}
}
func (r roundTripRefResource) MarshalRef(name string) []ResourceIdentifier {
	if name != r.refName {
		return nil
	}
	if r.refType == RelationToOne {
		if r.toOneID == "" {
			return nil
		}
		return []ResourceIdentifier{Ref{ID: r.toOneID, Type: r.toOneType}}
	}
	// to-many
	refs := make([]ResourceIdentifier, len(r.toManyRefs))
	for i, ref := range r.toManyRefs {
		refs[i] = ref
	}
	return refs
}
func (r *roundTripRefResource) UnmarshalRef(name string, id string, meta map[string]interface{}) error {
	if name != r.refName {
		return fmt.Errorf("unexpected relationship: %s", name)
	}
	if r.refType == RelationToOne {
		r.toOneID = id
		return nil
	}
	// to-many: accumulate refs (type is not passed via UnmarshalRef, so we track IDs).
	r.toManyRefs = append(r.toManyRefs, Ref{ID: id})
	return nil
}

// randomRelationshipInput holds randomly generated relationship data for property testing.
type randomRelationshipInput struct {
	IsToMany bool
	ID       string
	Type     string
	RefName  string
	// For to-one.
	ToOneID   string
	ToOneType string
	// For to-many.
	ToManyRefs []Ref
}

func (randomRelationshipInput) Generate(rand *rand.Rand, size int) reflect.Value {
	r := randomRelationshipInput{
		IsToMany: rand.Intn(2) == 0,
		ID:       alphanumInput(uint64(rand.Intn(100000)) + 1),
		Type:     alphanumInput(uint64(rand.Intn(100000)) + 1),
		RefName:  alphanumInput(uint64(rand.Intn(100000)) + 1),
	}

	if r.IsToMany {
		count := rand.Intn(5) + 1
		r.ToManyRefs = make([]Ref, count)
		for i := 0; i < count; i++ {
			r.ToManyRefs[i] = Ref{
				ID:   alphanumInput(uint64(rand.Intn(100000)) + 1),
				Type: alphanumInput(uint64(rand.Intn(100000)) + 1),
			}
		}
	} else {
		r.ToOneID = alphanumInput(uint64(rand.Intn(100000)) + 1)
		r.ToOneType = alphanumInput(uint64(rand.Intn(100000)) + 1)
	}

	return reflect.ValueOf(r)
}

func TestRelationshipMarshalUnmarshalRoundTrip(t *testing.T) {
	config := &quick.Config{MaxCount: 100}

	t.Run("to-one relationship round-trip preserves resource identifier", func(t *testing.T) {
		f := func(input randomRelationshipInput) bool {
			if input.IsToMany {
				return true // skip to-many in this sub-test
			}

			original := &roundTripRefResource{
				id:        input.ID,
				typ:       input.Type,
				refName:   input.RefName,
				refType:   RelationToOne,
				toOneID:   input.ToOneID,
				toOneType: input.ToOneType,
			}

			// Marshal the relationship.
			data, err := MarshalRef(original, input.RefName)
			if err != nil {
				return false
			}

			// Unmarshal the relationship.
			result := &roundTripRefResource{
				id:      input.ID,
				typ:     input.Type,
				refName: input.RefName,
				refType: RelationToOne,
			}
			if err := UnmarshalRef(data, input.RefName, result); err != nil {
				return false
			}

			// Verify the to-one relationship ID is preserved.
			return result.toOneID == original.toOneID
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})

	t.Run("to-many relationship round-trip preserves resource identifiers", func(t *testing.T) {
		f := func(input randomRelationshipInput) bool {
			if !input.IsToMany {
				return true // skip to-one in this sub-test
			}

			original := &roundTripRefResource{
				id:         input.ID,
				typ:        input.Type,
				refName:    input.RefName,
				refType:    RelationToMany,
				toManyRefs: input.ToManyRefs,
			}

			// Marshal the relationship.
			data, err := MarshalRef(original, input.RefName)
			if err != nil {
				return false
			}

			// Unmarshal the relationship.
			result := &roundTripRefResource{
				id:      input.ID,
				typ:     input.Type,
				refName: input.RefName,
				refType: RelationToMany,
			}
			if err := UnmarshalRef(data, input.RefName, result); err != nil {
				return false
			}

			// Verify the to-many relationship IDs are preserved in order.
			if len(result.toManyRefs) != len(original.toManyRefs) {
				return false
			}
			for i, ref := range original.toManyRefs {
				if result.toManyRefs[i].ID != ref.ID {
					return false
				}
			}
			return true
		}
		err := quick.Check(f, config)
		assert.NoError(t, err)
	})
}

// Task 9.1: Integration test for full CRUD cycle
// Requirements: 3.1–3.5, 5.1–5.4, 6.1–6.5, 7.1–7.4

func TestIntegration_FullCRUDCycle(t *testing.T) {
	// Stateful server that maintains a map of resources keyed by ID.
	var mu sync.Mutex
	store := make(map[string]Resource)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")

		// Parse the path to determine the operation.
		// Expected paths: POST /articles, GET /articles/{id}, PATCH /articles/{id}, DELETE /articles/{id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

		switch r.Method {
		case http.MethodPost:
			// Create: POST /articles
			if len(parts) != 1 || parts[0] != "articles" {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}

			// Read the raw body and parse it to extract the resource data.
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request", Detail: err.Error()}}})
				return
			}

			// Parse the raw JSON to extract type and attributes from the data object.
			var raw struct {
				Data struct {
					Type       string          `json:"type"`
					Attributes json.RawMessage `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(bodyBytes, &raw); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request", Detail: err.Error()}}})
				return
			}

			// Build the resource with a server-generated ID.
			res := Resource{
				ID:         "server-1",
				Type:       raw.Data.Type,
				Attributes: raw.Data.Attributes,
			}
			mu.Lock()
			store[res.ID] = res
			mu.Unlock()

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: res}})

		case http.MethodGet:
			// Fetch: GET /articles/{id}
			if len(parts) != 2 || parts[0] != "articles" {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}
			id := parts[1]

			mu.Lock()
			res, ok := store[id]
			mu.Unlock()

			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found", Detail: "Resource " + id + " not found"}}})
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: res}})

		case http.MethodPatch:
			// Update: PATCH /articles/{id}
			if len(parts) != 2 || parts[0] != "articles" {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}
			id := parts[1]

			mu.Lock()
			_, ok := store[id]
			mu.Unlock()

			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}

			// Parse the raw JSON to extract type and attributes from the data object.
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request"}}})
				return
			}

			var raw struct {
				Data struct {
					Type       string          `json:"type"`
					Attributes json.RawMessage `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(bodyBytes, &raw); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request"}}})
				return
			}

			// Update the stored resource with the new attributes.
			updated := Resource{
				ID:         id,
				Type:       raw.Data.Type,
				Attributes: raw.Data.Attributes,
			}
			mu.Lock()
			store[id] = updated
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{one: updated}})

		case http.MethodDelete:
			// Delete: DELETE /articles/{id}
			if len(parts) != 2 || parts[0] != "articles" {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}
			id := parts[1]

			mu.Lock()
			_, ok := store[id]
			if ok {
				delete(store, id)
			}
			mu.Unlock()

			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
				return
			}

			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "405", Title: "Method Not Allowed"}}})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	// Step 1: Create a resource.
	createResource := clientTestResource{ID: "temp-id", Name: "Integration Article"}
	createResp, err := c.Create(ctx, createResource)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created clientTestResource
	err = createResp.Unmarshal(&created)
	assert.NoError(t, err)
	assert.Equal(t, "server-1", created.ID)
	assert.Equal(t, "Integration Article", created.Name)

	// Step 2: Fetch the created resource.
	fetchResp, err := c.Fetch(ctx, "articles", created.ID)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchResp.StatusCode)

	var fetched clientTestResource
	err = fetchResp.Unmarshal(&fetched)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Integration Article", fetched.Name)

	// Step 3: Update the resource.
	updateResource := clientTestResource{ID: created.ID, Name: "Updated Integration Article"}
	updateResp, err := c.Update(ctx, updateResource)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	var updated clientTestResource
	err = updateResp.Unmarshal(&updated)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Updated Integration Article", updated.Name)

	// Verify the update persisted by fetching again.
	fetchResp2, err := c.Fetch(ctx, "articles", created.ID)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchResp2.StatusCode)

	var fetched2 clientTestResource
	err = fetchResp2.Unmarshal(&fetched2)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Integration Article", fetched2.Name)

	// Step 4: Delete the resource.
	deleteResp, err := c.Delete(ctx, "articles", created.ID)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	// Verify the resource is gone.
	_, err = c.Fetch(ctx, "articles", created.ID)
	assert.Error(t, err)
	var respErr *ResponseError
	assert.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusNotFound, respErr.StatusCode)
	assert.True(t, respErr.HasErrors())
	assert.Equal(t, "Not Found", respErr.Errors()[0].Title)
}

// Task 9.2: Integration test for relationship operations
// Requirements: 8.1–8.3, 9.1–9.4, 10.1–10.4, 11.1–11.4

func TestIntegration_RelationshipOperations(t *testing.T) {
	// Stateful server that maintains relationship data for a resource.
	// The resource "articles/1" has a to-many relationship "tags".
	var mu sync.Mutex
	// tagRefs stores the current set of tag references for articles/1.
	tagRefs := []Resource{
		{ID: "t1", Type: "tags"},
		{ID: "t2", Type: "tags"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")

		// Expected paths: /articles/1/relationships/tags
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

		// Validate the path matches /articles/1/relationships/tags
		if len(parts) != 4 || parts[0] != "articles" || parts[1] != "1" || parts[2] != "relationships" || parts[3] != "tags" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "404", Title: "Not Found"}}})
			return
		}

		switch r.Method {
		case http.MethodGet:
			// FetchRef: return current tag references.
			mu.Lock()
			refs := make([]Resource, len(tagRefs))
			copy(refs, tagRefs)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{isMany: true, many: refs}})

		case http.MethodPatch:
			// UpdateRef: replace the entire set of tag references.
			var doc Document
			if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request"}}})
				return
			}

			mu.Lock()
			tagRefs = make([]Resource, len(doc.Data.many))
			copy(tagRefs, doc.Data.many)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			mu.Lock()
			refs := make([]Resource, len(tagRefs))
			copy(refs, tagRefs)
			mu.Unlock()
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{isMany: true, many: refs}})

		case http.MethodPost:
			// AddRef: append new tag references.
			var doc Document
			if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request"}}})
				return
			}

			mu.Lock()
			tagRefs = append(tagRefs, doc.Data.many...)
			refs := make([]Resource, len(tagRefs))
			copy(refs, tagRefs)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Document{Data: &DocumentData{isMany: true, many: refs}})

		case http.MethodDelete:
			// RemoveRef: remove specified tag references.
			var doc Document
			if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "400", Title: "Bad Request"}}})
				return
			}

			// Build a set of IDs to remove.
			removeIDs := make(map[string]bool)
			for _, res := range doc.Data.many {
				removeIDs[res.ID] = true
			}

			mu.Lock()
			var remaining []Resource
			for _, ref := range tagRefs {
				if !removeIDs[ref.ID] {
					remaining = append(remaining, ref)
				}
			}
			tagRefs = remaining
			mu.Unlock()

			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Document{Errors: []*Error{{Status: "405", Title: "Method Not Allowed"}}})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	// Step 1: FetchRef — verify initial state has 2 tags.
	fetchRefResp, err := c.FetchRef(ctx, "articles", "1", "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchRefResp.StatusCode)
	assert.NotNil(t, fetchRefResp.Document())
	assert.NotNil(t, fetchRefResp.Document().Data)
	assert.Len(t, fetchRefResp.Document().Data.many, 2)
	assert.Equal(t, "t1", fetchRefResp.Document().Data.many[0].ID)
	assert.Equal(t, "t2", fetchRefResp.Document().Data.many[1].ID)

	// Step 2: UpdateRef — replace tags with a single new tag.
	updateRefResource := clientRefTestResource{id: "1", typ: "articles", refName: "tags", refType: RelationToMany}
	updateRefResp, err := c.UpdateRef(ctx, updateRefResource, "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateRefResp.StatusCode)

	// Verify the update by fetching again.
	fetchRefResp2, err := c.FetchRef(ctx, "articles", "1", "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchRefResp2.StatusCode)
	// UpdateRef sent the marshaled ref from clientRefTestResource which has one ref: {id: "ref1", type: "articles_ref"}
	assert.Len(t, fetchRefResp2.Document().Data.many, 1)
	assert.Equal(t, "ref1", fetchRefResp2.Document().Data.many[0].ID)

	// Step 3: AddRef — add another tag reference.
	addRefResource := clientRefTestResource{id: "1", typ: "articles", refName: "tags", refType: RelationToMany}
	addRefResp, err := c.AddRef(ctx, addRefResource, "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, addRefResp.StatusCode)

	// Verify the addition by fetching again.
	fetchRefResp3, err := c.FetchRef(ctx, "articles", "1", "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchRefResp3.StatusCode)
	// Should now have 2 refs: the one from UpdateRef + the one from AddRef.
	assert.Len(t, fetchRefResp3.Document().Data.many, 2)

	// Step 4: RemoveRef — remove the first tag reference.
	removeRefResource := clientRefTestResource{id: "1", typ: "articles", refName: "tags", refType: RelationToMany}
	removeRefResp, err := c.RemoveRef(ctx, removeRefResource, "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, removeRefResp.StatusCode)

	// Verify the removal by fetching again.
	fetchRefResp4, err := c.FetchRef(ctx, "articles", "1", "tags")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, fetchRefResp4.StatusCode)
	// RemoveRef sent the same ref {id: "ref1", type: "articles_ref"}, so both refs with ID "ref1" should be removed.
	// The remaining refs should be those that don't have ID "ref1".
	for _, ref := range fetchRefResp4.Document().Data.many {
		assert.NotEqual(t, "ref1", ref.ID, "ref1 should have been removed")
	}
}

// Task 9.3: Integration test for PageIterator multi-page traversal
// Requirements: 14.1–14.8

func TestIntegration_PageIteratorMultiPageTraversal(t *testing.T) {
	// Mock server serving 3 pages of articles.
	// Page 1: articles 1, 2 (next → page 2)
	// Page 2: articles 3, 4 (next → page 3)
	// Page 3: article 5 (no next link)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)

		page := r.URL.Query().Get("page")
		switch page {
		case "2":
			attrs3, _ := json.Marshal(map[string]string{"name": "Article 3"})
			attrs4, _ := json.Marshal(map[string]string{"name": "Article 4"})
			doc := Document{
				Data: &DocumentData{
					isMany: true,
					many: []Resource{
						{ID: "3", Type: "articles", Attributes: attrs3},
						{ID: "4", Type: "articles", Attributes: attrs4},
					},
				},
				Links: map[string]Link{
					"next": {Href: "http://" + r.Host + "/articles?page=3"},
				},
			}
			json.NewEncoder(w).Encode(doc)

		case "3":
			attrs5, _ := json.Marshal(map[string]string{"name": "Article 5"})
			doc := Document{
				Data: &DocumentData{
					isMany: true,
					many: []Resource{
						{ID: "5", Type: "articles", Attributes: attrs5},
					},
				},
				// No "next" link — last page.
			}
			json.NewEncoder(w).Encode(doc)

		default:
			// Page 1 (initial request from List).
			attrs1, _ := json.Marshal(map[string]string{"name": "Article 1"})
			attrs2, _ := json.Marshal(map[string]string{"name": "Article 2"})
			doc := Document{
				Data: &DocumentData{
					isMany: true,
					many: []Resource{
						{ID: "1", Type: "articles", Attributes: attrs1},
						{ID: "2", Type: "articles", Attributes: attrs2},
					},
				},
				Links: map[string]Link{
					"next": {Href: "http://" + r.Host + "/articles?page=2"},
				},
			}
			json.NewEncoder(w).Encode(doc)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	// Fetch the first page via List.
	listResp, err := c.List(ctx, "articles")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	// Collect all items across all pages using PageIterator.
	var allItems []clientTestResource

	// Unmarshal page 1.
	var page1Items []clientTestResource
	err = listResp.Unmarshal(&page1Items)
	assert.NoError(t, err)
	allItems = append(allItems, page1Items...)

	// Iterate through remaining pages.
	iter := c.Pages(listResp)
	pageCount := 1
	for iter.Next(ctx) {
		pageCount++
		var pageItems []clientTestResource
		err := iter.Items(&pageItems)
		assert.NoError(t, err)
		allItems = append(allItems, pageItems...)
	}
	assert.NoError(t, iter.Err())

	// Verify all 3 pages were consumed.
	assert.Equal(t, 3, pageCount)

	// Verify all 5 items were collected.
	assert.Len(t, allItems, 5)
	assert.Equal(t, "1", allItems[0].ID)
	assert.Equal(t, "Article 1", allItems[0].Name)
	assert.Equal(t, "2", allItems[1].ID)
	assert.Equal(t, "Article 2", allItems[1].Name)
	assert.Equal(t, "3", allItems[2].ID)
	assert.Equal(t, "Article 3", allItems[2].Name)
	assert.Equal(t, "4", allItems[3].ID)
	assert.Equal(t, "Article 4", allItems[3].Name)
	assert.Equal(t, "5", allItems[4].ID)
	assert.Equal(t, "Article 5", allItems[4].Name)
}
