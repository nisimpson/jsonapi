package jsonapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test types for integration tests
type Article struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	AuthorID string   `json:"-"`
	TagIDs   []string `json:"-"`
}

func (a Article) ResourceID() string   { return a.ID }
func (a Article) ResourceType() string { return "articles" }

func (a *Article) SetResourceID(id string) error {
	a.ID = id
	return nil
}

func (a Article) Relationships() map[string]RelationType {
	return map[string]RelationType{
		"author": RelationToOne,
		"tags":   RelationToMany,
	}
}

func (a Article) MarshalRef(name string) []ResourceIdentifier {
	switch name {
	case "author":
		if a.AuthorID != "" {
			return []ResourceIdentifier{User{ID: a.AuthorID}}
		}
	case "tags":
		var refs []ResourceIdentifier
		for _, tag := range a.Tags() {
			refs = append(refs, tag)
		}
		return refs
	}
	return nil
}

func (a Article) Tags() []Tag {
	var tags []Tag
	for _, id := range a.TagIDs {
		tags = append(tags, Tag{ID: id})
	}
	return tags
}

func (a *Article) UnmarshalRef(name, id string, meta map[string]interface{}) error {
	switch name {
	case "author":
		a.AuthorID = id
	case "tags":
		a.TagIDs = append(a.TagIDs, id)
	}
	return nil
}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) ResourceID() string   { return u.ID }
func (u User) ResourceType() string { return "users" }

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (t Tag) ResourceID() string   { return t.ID }
func (t Tag) ResourceType() string { return "tags" }

// Test data
var articles = map[string]Article{
	"1": {ID: "1", Title: "First Article", Content: "Content of first article", AuthorID: "1", TagIDs: []string{"1", "2"}},
	"2": {ID: "2", Title: "Second Article", Content: "Content of second article", AuthorID: "2"},
}

var users = map[string]User{
	"1": {ID: "1", Name: "John Doe"},
	"2": {ID: "2", Name: "Jane Smith"},
}

var tags = map[string]Tag{
	"1": {ID: "1", Name: "Go"},
	"2": {ID: "2", Name: "JSON:API"},
}

func TestContext_UnmarshalData(t *testing.T) {
	// Create a test request with JSON:API data
	body := `{
		"data": {
			"type": "articles",
			"attributes": {
				"title": "Test Article"
			}
		}
	}`

	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/vnd.api+json")

	// Create a Request instance
	jsonapiReq := &Context{}

	// Test UnmarshalData
	var resource Article
	err := jsonapiReq.Unmarshal(req.Body, &resource)
	require.NoError(t, err)

	assert.Equal(t, "", resource.ID) // No ID in creation request
	assert.Equal(t, "Test Article", resource.Title)
}

func TestContext_UnmarshalRef(t *testing.T) {
	// Create a test request with relationship data
	body := `{
		"data": {
			"id": "123",
			"type": "users"
		}
	}`

	req := httptest.NewRequest("PATCH", "/articles/1/relationships/author", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/vnd.api+json")

	// Create a Request instance
	jsonapiReq := &Context{}

	// Test UnmarshalRef
	var resource Article
	err := jsonapiReq.UnmarshalRef(req.Body, "author", &resource)
	require.NoError(t, err)

	assert.Equal(t, "123", resource.AuthorID)
}

func TestContext_MarshalRef(t *testing.T) {
	ctx := &Context{}

	article := Article{
		ID:       "1",
		Title:    "Test Article",
		AuthorID: "123",
		TagIDs:   []string{"1", "2"},
	}

	t.Run("marshal author relationship", func(t *testing.T) {
		w := httptest.NewRecorder()
		n, err := ctx.MarshalRef(w, http.StatusOK, "author", article)
		require.NoError(t, err)
		assert.Greater(t, n, 0)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("marshal tags relationship", func(t *testing.T) {
		w := httptest.NewRecorder()
		n, err := ctx.MarshalRef(w, http.StatusOK, "tags", article)
		require.NoError(t, err)
		assert.Greater(t, n, 0)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid relationship name", func(t *testing.T) {
		w := httptest.NewRecorder()
		_, err := ctx.MarshalRef(w, http.StatusOK, "invalid", article)
		assert.Error(t, err)
	})
}

func TestDefaultServeMux(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	type testcase struct {
		name       string
		path       string
		handler    http.HandlerFunc
		method     string
		wantStatus int
	}

	for _, tc := range []testcase{
		{
			name:       "articles",
			method:     "POST",
			path:       "/articles",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles with id",
			method:     "GET",
			path:       "/articles/1",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles get author ref",
			method:     "GET",
			path:       "/articles/1/relationships/author",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles update author ref",
			method:     "PATCH",
			path:       "/articles/1/relationships/author",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles add tag ref",
			method:     "POST",
			path:       "/articles/1/relationships/tags",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles delete tag ref",
			method:     "DELETE",
			path:       "/articles/1/relationships/tags",
			handler:    handler,
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles tag ref - unsupported method",
			method:     "PUT",
			path:       "/articles/1/relationships/tags",
			handler:    handler,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "users",
			method:     "GET",
			path:       "/users",
			handler:    handler,
			wantStatus: http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			mux := DefaultServeMux(map[string]ResourceHandler{
				"articles": {
					Retrieve: tc.handler,
					Create:   tc.handler,
					Update:   tc.handler,
					Delete:   tc.handler,
					Refs: RelationshipHandlerMux{
						"author": {
							Get:    tc.handler,
							Del:    tc.handler,
							Add:    tc.handler,
							Update: tc.handler,
						},
						"tags": {
							Get:    tc.handler,
							Del:    tc.handler,
							Add:    tc.handler,
							Update: tc.handler,
						},
					},
				},
			})
			mux.ServeHTTP(w, req)
			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestResourceHandler(t *testing.T) {
	type testcase struct {
		name       string
		ctx        Context
		method     string
		path       string
		wantStatus int
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []testcase{
		{
			name:       "create article",
			method:     "POST",
			path:       "/articles",
			ctx:        Context{ResourceType: "articles"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "list articles",
			method:     "GET",
			path:       "/articles",
			ctx:        Context{ResourceType: "articles"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles with id",
			method:     "GET",
			path:       "/articles/1",
			ctx:        Context{ResourceType: "articles", ResourceID: "1"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "update article",
			method:     "PATCH",
			path:       "/articles/1",
			ctx:        Context{ResourceType: "articles", ResourceID: "1"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "delete article",
			method:     "DELETE",
			path:       "/articles/1",
			ctx:        Context{ResourceType: "articles", ResourceID: "1"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles get author ref",
			method:     "GET",
			path:       "/articles/1/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles get author relation",
			method:     "GET",
			path:       "/articles/1/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author", Related: true},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles update author ref",
			method:     "PATCH",
			path:       "/articles/1/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles add tag ref",
			method:     "POST",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles delete tag ref",
			method:     "DELETE",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "articles tag ref - unsupported method",
			method:     "PUT",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing resource type",
			method:     "GET",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			handler := ResourceHandler{
				Create:   handler,
				Retrieve: handler,
				Update:   handler,
				Delete:   handler,
				List:     handler,
				Refs:     handler,
			}
			handler.ServeHTTP(w, req.WithContext(WithContext(req.Context(), &tc.ctx)))
			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestRelationshipHandler(t *testing.T) {
	type testcase struct {
		name       string
		ctx        Context
		method     string
		path       string
		wantStatus int
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []testcase{
		{
			name:       "get relationship",
			method:     "GET",
			path:       "/articles/1/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "update relationship",
			method:     "PATCH",
			path:       "/articles/1/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "add to relationship",
			method:     "POST",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "delete from relationship",
			method:     "DELETE",
			path:       "/articles/1/relationships/tags",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unsupported method",
			method:     "PUT",
			path:       "/articles/1/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing relationship",
			method:     "GET",
			path:       "/articles/1/relationships/",
			ctx:        Context{ResourceType: "articles", ResourceID: "1", Relationship: ""},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing resource id",
			method:     "GET",
			path:       "/articles/relationships/author",
			ctx:        Context{ResourceType: "articles", ResourceID: "", Relationship: "author"},
			wantStatus: http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			handler := RelationshipHandler{
				Get:    handler,
				Update: handler,
				Add:    handler,
				Del:    handler,
			}
			handler.ServeHTTP(w, req.WithContext(WithContext(req.Context(), &tc.ctx)))
			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestWithContext(t *testing.T) {
	ctx := context.Background()
	request := &Context{ResourceType: "articles", ResourceID: "1"}

	newCtx := WithContext(ctx, request)
	assert.NotEqual(t, ctx, newCtx)

	retrieved := FromContext(newCtx)
	assert.Equal(t, request, retrieved)
}

func TestFromContext(t *testing.T) {
	t.Run("with context", func(t *testing.T) {
		ctx := context.Background()
		request := &Context{ResourceType: "articles", ResourceID: "1"}
		newCtx := WithContext(ctx, request)

		retrieved := FromContext(newCtx)
		assert.Equal(t, request, retrieved)
	})

	t.Run("without context", func(t *testing.T) {
		ctx := context.Background()
		retrieved := FromContext(ctx)
		assert.Equal(t, &Context{}, retrieved)
	})
}

func TestContext_Marshal(t *testing.T) {
	ctx := &Context{}
	w := httptest.NewRecorder()

	article := Article{ID: "1", Title: "Test"}

	n, err := ctx.Marshal(w, http.StatusOK, article)
	require.NoError(t, err)
	assert.Greater(t, n, 0)
	assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContext_MarshalErrors(t *testing.T) {
	ctx := &Context{}
	w := httptest.NewRecorder()

	err1 := &Error{Status: "400", Title: "Bad Request"}
	err2 := &Error{Status: "422", Title: "Validation Error"}

	n, err := ctx.MarshalErrors(w, http.StatusBadRequest, err1, err2)
	require.NoError(t, err)
	assert.Greater(t, n, 0)
	assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandle(t *testing.T) {
	resolver := DefaultRequestResolver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := FromContext(r.Context())
		assert.Equal(t, "articles", ctx.ResourceType)
		assert.Equal(t, "1", ctx.ResourceID)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := Handle(resolver, handler)

	req := httptest.NewRequest("GET", "/articles/1", nil)
	req.SetPathValue("type", "articles")
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDefaultRequestResolver_ResolveJSONAPIRequest(t *testing.T) {
	resolver := DefaultRequestResolver{}

	t.Run("basic resource request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1", nil)
		req.SetPathValue("type", "articles")
		req.SetPathValue("id", "1")

		ctx := resolver.ResolveJSONAPIRequest(req)
		assert.Equal(t, "articles", ctx.ResourceType)
		assert.Equal(t, "1", ctx.ResourceID)
		assert.Equal(t, "", ctx.Relationship)
		assert.False(t, ctx.Related)
	})

	t.Run("relationship request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1/relationships/author", nil)
		req.SetPathValue("type", "articles")
		req.SetPathValue("id", "1")
		req.SetPathValue("ref", "author")

		ctx := resolver.ResolveJSONAPIRequest(req)
		assert.Equal(t, "articles", ctx.ResourceType)
		assert.Equal(t, "1", ctx.ResourceID)
		assert.Equal(t, "author", ctx.Relationship)
		assert.False(t, ctx.Related)
	})

	t.Run("related resource request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1/author", nil)
		req.SetPathValue("type", "articles")
		req.SetPathValue("id", "1")
		req.SetPathValue("related", "author")

		ctx := resolver.ResolveJSONAPIRequest(req)
		assert.Equal(t, "articles", ctx.ResourceType)
		assert.Equal(t, "1", ctx.ResourceID)
		assert.Equal(t, "author", ctx.Relationship)
		assert.True(t, ctx.Related)
	})
}

func TestResourceHandlerMux_ServeHTTP(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := ResourceHandlerMux{
		"articles": ResourceHandler{Retrieve: handler},
	}

	t.Run("valid resource type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1", nil)
		ctx := &Context{ResourceType: "articles", ResourceID: "1"}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid resource type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/1", nil)
		ctx := &Context{ResourceType: "users", ResourceID: "1"}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing resource type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := &Context{ResourceType: ""}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestRelationshipHandlerMux_ServeHTTP(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := RelationshipHandlerMux{
		"author": RelationshipHandler{Get: handler},
	}

	t.Run("valid relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1/relationships/author", nil)
		ctx := &Context{ResourceType: "articles", ResourceID: "1", Relationship: "author"}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1/relationships/tags", nil)
		ctx := &Context{ResourceType: "articles", ResourceID: "1", Relationship: "tags"}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/1/relationships/", nil)
		ctx := &Context{ResourceType: "articles", ResourceID: "1", Relationship: ""}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing resource id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/articles/relationships/author", nil)
		ctx := &Context{ResourceType: "articles", ResourceID: "", Relationship: "author"}
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req.WithContext(WithContext(req.Context(), ctx)))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestError_Error(t *testing.T) {
	err := &Error{
		Status: "400",
		Title:  "Bad Request",
		Detail: "Invalid input",
	}

	result := err.Error()
	assert.Contains(t, result, "Bad Request")
	assert.Contains(t, result, "Invalid input")
}

func TestContext_UnmarshalErrors(t *testing.T) {
	ctx := &Context{}

	t.Run("io error", func(t *testing.T) {
		var target Article
		err := ctx.Unmarshal(strings.NewReader("invalid json"), &target)
		assert.Error(t, err)
	})

	t.Run("unmarshal ref io error", func(t *testing.T) {
		var target Article
		err := ctx.UnmarshalRef(strings.NewReader("invalid json"), "author", &target)
		assert.Error(t, err)
	})
}

func TestWriteError(t *testing.T) {
	w := &errorWriter{}
	_, err := write(w, http.StatusOK, Article{ID: "1"})
	assert.Error(t, err)
}

func TestTryServeHTTPWithNilHandler(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	tryServeHTTP(w, req, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

type errorWriter struct{}

func (e *errorWriter) Header() http.Header        { return make(http.Header) }
func (e *errorWriter) WriteHeader(statusCode int) {}
func (e *errorWriter) Write([]byte) (int, error)  { return 0, assert.AnError }
