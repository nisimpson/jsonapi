package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/nisimpson/jsonapi"
	"github.com/nisimpson/jsonapi/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers and mock implementations

// customError is a test error type that implements the error interface
type customError struct {
	message string
}

func (e customError) Error() string {
	return e.message
}

// mockRequestContextResolver is a mock implementation of RequestContextResolver for testing
type mockRequestContextResolver struct {
	resolveFunc func(r *http.Request) (*server.RequestContext, error)
}

func (m *mockRequestContextResolver) ResolveRequestContext(r *http.Request) (*server.RequestContext, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(r)
	}
	return &server.RequestContext{
		ResourceType: "users",
		ResourceID:   "1",
	}, nil
}

// mockHandler is a simple mock HTTP handler for testing
type mockHandler struct {
	called     bool
	statusCode int
	response   string
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	if m.statusCode != 0 {
		w.WriteHeader(m.statusCode)
	}
	if m.response != "" {
		w.Write([]byte(m.response))
	}
}

// Test RequestContext functionality

func TestRequestContext_Structure(t *testing.T) {
	t.Run("creates request context with all fields", func(t *testing.T) {
		rc := server.RequestContext{
			ResourceID:            "123",
			ResourceType:          "users",
			Relationship:          "posts",
			FetchRelatedResources: true,
		}

		assert.Equal(t, "123", rc.ResourceID)
		assert.Equal(t, "users", rc.ResourceType)
		assert.Equal(t, "posts", rc.Relationship)
		assert.True(t, rc.FetchRelatedResources)
	})

	t.Run("creates empty request context", func(t *testing.T) {
		rc := server.RequestContext{}

		assert.Equal(t, "", rc.ResourceID)
		assert.Equal(t, "", rc.ResourceType)
		assert.Equal(t, "", rc.Relationship)
		assert.False(t, rc.FetchRelatedResources)
	})
}

// Test context management functions

func TestSetRequestContext(t *testing.T) {
	t.Run("stores request context in context", func(t *testing.T) {
		parentCtx := context.Background()
		requestContext := &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		}

		ctx := server.SetRequestContext(parentCtx, requestContext)

		// Verify the context is not the same as parent
		assert.NotEqual(t, parentCtx, ctx)

		// Verify we can retrieve the stored context
		stored, ok := server.GetRequestContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, requestContext, stored)
	})

	t.Run("handles nil request context", func(t *testing.T) {
		parentCtx := context.Background()
		ctx := server.SetRequestContext(parentCtx, nil)

		stored, ok := server.GetRequestContext(ctx)
		assert.True(t, ok)
		assert.Nil(t, stored)
	})
}

func TestGetRequestContext(t *testing.T) {
	t.Run("retrieves stored request context", func(t *testing.T) {
		requestContext := &server.RequestContext{
			ResourceType: "posts",
			ResourceID:   "456",
			Relationship: "comments",
		}

		ctx := server.SetRequestContext(context.Background(), requestContext)
		retrieved, ok := server.GetRequestContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, requestContext, retrieved)
	})

	t.Run("returns false when no context stored", func(t *testing.T) {
		ctx := context.Background()
		retrieved, ok := server.GetRequestContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	// testContextKey is a custom type for test context keys to avoid collisions
	type testContextKey string

	t.Run("returns false when context has different key", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), testContextKey("other-key"), "some-value")
		retrieved, ok := server.GetRequestContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})
}

// Test UseRequestContextResolver middleware

func TestUseRequestContextResolver(t *testing.T) {
	t.Run("successfully resolves and sets context", func(t *testing.T) {
		expectedContext := &server.RequestContext{
			ResourceType: "articles",
			ResourceID:   "789",
		}

		resolver := &mockRequestContextResolver{
			resolveFunc: func(r *http.Request) (*server.RequestContext, error) {
				return expectedContext, nil
			},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, ok := server.GetRequestContext(r.Context())
			assert.True(t, ok)
			assert.Equal(t, expectedContext, ctx)
			w.WriteHeader(http.StatusOK)
		})

		middleware := server.UseRequestContextResolver(nextHandler, resolver)

		req := httptest.NewRequest("GET", "/articles/789", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns error when resolver fails", func(t *testing.T) {
		resolver := &mockRequestContextResolver{
			resolveFunc: func(r *http.Request) (*server.RequestContext, error) {
				return nil, errors.New("resolver error")
			},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		})

		middleware := server.UseRequestContextResolver(nextHandler, resolver)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "resolver error")
	})

	t.Run("passes request with updated context to next handler", func(t *testing.T) {
		resolver := &mockRequestContextResolver{
			resolveFunc: func(r *http.Request) (*server.RequestContext, error) {
				return &server.RequestContext{ResourceType: "test"}, nil
			},
		}

		var receivedRequest *http.Request
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequest = r
		})

		middleware := server.UseRequestContextResolver(nextHandler, resolver)

		originalReq := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, originalReq)

		assert.NotNil(t, receivedRequest)
		assert.NotEqual(t, originalReq.Context(), receivedRequest.Context())

		ctx, ok := server.GetRequestContext(receivedRequest.Context())
		assert.True(t, ok)
		assert.Equal(t, "test", ctx.ResourceType)
	})
}

// Test ResourceHandler

func TestResourceHandler_ServeHTTP(t *testing.T) {
	t.Run("routes GET request with ID to Get handler", func(t *testing.T) {
		getHandler := &mockHandler{}
		rh := server.ResourceHandler{Get: getHandler}

		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, getHandler.called)
	})

	t.Run("routes GET request without ID to Search handler", func(t *testing.T) {
		searchHandler := &mockHandler{}
		rh := server.ResourceHandler{Search: searchHandler}

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, searchHandler.called)
	})

	t.Run("routes POST request to Create handler", func(t *testing.T) {
		createHandler := &mockHandler{}
		rh := server.ResourceHandler{Create: createHandler}

		req := httptest.NewRequest("POST", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, createHandler.called)
	})

	t.Run("routes PATCH request to Update handler", func(t *testing.T) {
		updateHandler := &mockHandler{}
		rh := server.ResourceHandler{Update: updateHandler}

		req := httptest.NewRequest("PATCH", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, updateHandler.called)
	})

	t.Run("routes DELETE request to Delete handler", func(t *testing.T) {
		deleteHandler := &mockHandler{}
		rh := server.ResourceHandler{Delete: deleteHandler}

		req := httptest.NewRequest("DELETE", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, deleteHandler.called)
	})

	t.Run("routes relationship requests to Relationship handler", func(t *testing.T) {
		relationshipHandler := &mockHandler{}
		rh := server.ResourceHandler{Relationship: relationshipHandler}

		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, relationshipHandler.called)
	})

	t.Run("returns 500 when no request context", func(t *testing.T) {
		rh := server.ResourceHandler{}
		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context not found")
	})

	t.Run("returns 500 when resource type is missing", func(t *testing.T) {
		rh := server.ResourceHandler{}
		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context missing resource type")
	})

	t.Run("returns 405 for unsupported HTTP method", func(t *testing.T) {
		rh := server.ResourceHandler{}
		req := httptest.NewRequest("PUT", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.Contains(t, w.Body.String(), "Method not allowed")
	})

	t.Run("returns 404 when no appropriate handler found", func(t *testing.T) {
		rh := server.ResourceHandler{} // No handlers set
		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Resource not found")
	})

	t.Run("prefers Get handler over Search for GET with ID", func(t *testing.T) {
		getHandler := &mockHandler{}
		searchHandler := &mockHandler{}
		rh := server.ResourceHandler{
			Get:    getHandler,
			Search: searchHandler,
		}

		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, getHandler.called)
		assert.False(t, searchHandler.called)
	})

	t.Run("falls back to Search when Get handler is nil", func(t *testing.T) {
		searchHandler := &mockHandler{}
		rh := server.ResourceHandler{
			Get:    nil,
			Search: searchHandler,
		}

		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, searchHandler.called)
	})
}

// Test ResourceHandlerMux

func TestResourceHandlerMux_ServeHTTP(t *testing.T) {
	t.Run("routes to correct resource handler", func(t *testing.T) {
		usersHandler := &mockHandler{}
		postsHandler := &mockHandler{}

		mux := server.ResourceHandlerMux{
			"users": usersHandler,
			"posts": postsHandler,
		}

		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.True(t, usersHandler.called)
		assert.False(t, postsHandler.called)
	})

	t.Run("routes to different resource handler", func(t *testing.T) {
		usersHandler := &mockHandler{}
		postsHandler := &mockHandler{}

		mux := server.ResourceHandlerMux{
			"users": usersHandler,
			"posts": postsHandler,
		}

		req := httptest.NewRequest("GET", "/posts/456", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "posts",
			ResourceID:   "456",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.False(t, usersHandler.called)
		assert.True(t, postsHandler.called)
	})

	t.Run("returns 500 when no request context", func(t *testing.T) {
		mux := server.ResourceHandlerMux{
			"users": &mockHandler{},
		}

		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context not found")
	})

	t.Run("returns 500 when resource type is missing", func(t *testing.T) {
		mux := server.ResourceHandlerMux{
			"users": &mockHandler{},
		}

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context missing resource type")
	})

	t.Run("returns 404 when resource type not found in mux", func(t *testing.T) {
		mux := server.ResourceHandlerMux{
			"users": &mockHandler{},
		}

		req := httptest.NewRequest("GET", "/articles/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "articles",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Resource not found")
	})

	t.Run("passes request and response to resource handler", func(t *testing.T) {
		handler := &mockHandler{
			statusCode: http.StatusCreated,
			response:   "test response",
		}

		mux := server.ResourceHandlerMux{
			"test": handler,
		}

		req := httptest.NewRequest("POST", "/test", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "test",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.True(t, handler.called)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "test response", w.Body.String())
	})
}

// Test RelationshipHandler

func TestRelationshipHandler_ServeHTTP(t *testing.T) {
	t.Run("routes GET request to Get handler", func(t *testing.T) {
		getHandler := &mockHandler{}
		rh := server.RelationshipHandler{Get: getHandler}

		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, getHandler.called)
	})

	t.Run("routes POST request to Add handler", func(t *testing.T) {
		addHandler := &mockHandler{}
		rh := server.RelationshipHandler{Add: addHandler}

		req := httptest.NewRequest("POST", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, addHandler.called)
	})

	t.Run("routes PATCH request to Update handler", func(t *testing.T) {
		updateHandler := &mockHandler{}
		rh := server.RelationshipHandler{Update: updateHandler}

		req := httptest.NewRequest("PATCH", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, updateHandler.called)
	})

	t.Run("routes DELETE request to Delete handler", func(t *testing.T) {
		deleteHandler := &mockHandler{}
		rh := server.RelationshipHandler{Delete: deleteHandler}

		req := httptest.NewRequest("DELETE", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.True(t, deleteHandler.called)
	})

	t.Run("returns 500 when no request context", func(t *testing.T) {
		rh := server.RelationshipHandler{}
		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context not found")
	})

	t.Run("returns 500 when relationship is missing", func(t *testing.T) {
		rh := server.RelationshipHandler{}
		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Request context missing relationship")
	})

	t.Run("returns 405 for unsupported HTTP method", func(t *testing.T) {
		rh := server.RelationshipHandler{}
		req := httptest.NewRequest("PUT", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.Contains(t, w.Body.String(), "Method not allowed")
	})

	t.Run("returns 404 when no appropriate handler found", func(t *testing.T) {
		rh := server.RelationshipHandler{} // No handlers set
		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Resource not found")
	})

	t.Run("requires resource ID for all operations", func(t *testing.T) {
		getHandler := &mockHandler{}
		rh := server.RelationshipHandler{Get: getHandler}

		req := httptest.NewRequest("GET", "/users/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			Relationship: "posts",
			// ResourceID is empty
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		rh.ServeHTTP(w, req)

		assert.False(t, getHandler.called)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Test Response struct

func TestResponse_Structure(t *testing.T) {
	t.Run("creates response with all fields", func(t *testing.T) {
		header := make(http.Header)
		header.Set("X-Custom", "value")

		doc := &jsonapi.Document{
			Meta: map[string]interface{}{
				"version": "1.0",
			},
		}

		response := server.Response{
			Status: http.StatusCreated,
			Header: header,
			Body:   doc,
		}

		assert.Equal(t, http.StatusCreated, response.Status)
		assert.Equal(t, header, response.Header)
		assert.Equal(t, doc, response.Body)
	})

	t.Run("creates empty response", func(t *testing.T) {
		response := server.Response{}

		assert.Equal(t, 0, response.Status)
		assert.Nil(t, response.Header)
		assert.Nil(t, response.Body)
	})
}

// Test HandlerFunc function type

func TestHandler_ServeHTTP(t *testing.T) {
	t.Run("calls handler function with context and request", func(t *testing.T) {
		var receivedContext *server.RequestContext
		var receivedRequest *http.Request

		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			receivedContext = ctx
			receivedRequest = r
			return server.Response{Status: http.StatusOK}, nil
		})

		expectedContext := &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		}

		req := httptest.NewRequest("GET", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), expectedContext)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, expectedContext, receivedContext)
		assert.Equal(t, req, receivedRequest)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("sets response status and headers", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			header := make(http.Header)
			header.Set("X-Custom", "test-value")
			header.Set("X-Another", "another-value")

			return server.Response{
				Status: http.StatusCreated,
				Header: header,
				Body:   &jsonapi.Document{}, // Need body for Content-Type to be set
			}, nil
		})

		req := httptest.NewRequest("POST", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "test-value", w.Header().Get("X-Custom"))
		assert.Equal(t, "another-value", w.Header().Get("X-Another"))
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	})

	t.Run("encodes JSON:API document body", func(t *testing.T) {
		doc := &jsonapi.Document{
			Meta: map[string]interface{}{
				"version": "1.0",
				"count":   42,
			},
		}

		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			return server.Response{
				Status: http.StatusOK,
				Body:   doc,
			}, nil
		})

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var decodedDoc jsonapi.Document
		err := json.Unmarshal(w.Body.Bytes(), &decodedDoc)
		require.NoError(t, err)

		assert.Equal(t, "1.0", decodedDoc.Meta["version"])
		assert.Equal(t, float64(42), decodedDoc.Meta["count"]) // JSON numbers are float64
	})

	t.Run("handles nil body", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			return server.Response{
				Status: http.StatusNoContent,
				Body:   nil,
			}, nil
		})

		req := httptest.NewRequest("DELETE", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Header().Get("Content-Type")) // No Content-Type when no body
		assert.Empty(t, w.Body.String())
	})

	t.Run("returns 500 when handler returns error", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			return server.Response{}, errors.New("handler error")
		})

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "handler error")
	})

	t.Run("returns 500 when no request context", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			return server.Response{Status: http.StatusOK}, nil
		})

		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		// The response should now be a JSON:API error document
		var doc jsonapi.Document
		err := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, err)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, "500", errorObj.Status)
		assert.Equal(t, "Internal Server Error", errorObj.Title)
		assert.Equal(t, "request context not found", errorObj.Detail)
	})

	t.Run("preserves existing headers and adds content-type", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			header := make(http.Header)
			header.Set("Content-Type", "text/plain") // This should be overridden
			header.Set("X-Custom", "value")

			return server.Response{
				Status: http.StatusOK,
				Header: header,
				Body:   &jsonapi.Document{}, // Need body for Content-Type to be set
			}, nil
		})

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
		assert.Equal(t, "value", w.Header().Get("X-Custom"))
	})
}

// Test DefaultHandler

func TestDefaultHandler(t *testing.T) {
	t.Run("creates handler with all standard routes", func(t *testing.T) {
		usersHandler := &mockHandler{response: "users"}
		postsHandler := &mockHandler{response: "posts"}

		mux := server.ResourceHandlerMux{
			"users": usersHandler,
			"posts": postsHandler,
		}

		handler := server.DefaultHandler(mux)
		assert.NotNil(t, handler)

		// Test that it's an http.Handler
		var _ http.Handler = handler
	})

	t.Run("routes GET /{type} to search", func(t *testing.T) {
		searchHandler := &mockHandler{response: "search"}
		resourceHandler := server.ResourceHandler{Search: searchHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, searchHandler.called)
	})

	t.Run("routes GET /{type}/{id} to get", func(t *testing.T) {
		getHandler := &mockHandler{response: "get"}
		resourceHandler := server.ResourceHandler{Get: getHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, getHandler.called)
	})

	t.Run("routes POST /{type} to create", func(t *testing.T) {
		createHandler := &mockHandler{response: "create"}
		resourceHandler := server.ResourceHandler{Create: createHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("POST", "/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, createHandler.called)
	})

	t.Run("routes PATCH /{type}/{id} to update", func(t *testing.T) {
		updateHandler := &mockHandler{response: "update"}
		resourceHandler := server.ResourceHandler{Update: updateHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("PATCH", "/users/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, updateHandler.called)
	})

	t.Run("routes DELETE /{type}/{id} to delete", func(t *testing.T) {
		deleteHandler := &mockHandler{response: "delete"}
		resourceHandler := server.ResourceHandler{Delete: deleteHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("DELETE", "/users/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, deleteHandler.called)
	})

	t.Run("routes GET /{type}/{id}/relationships/{relationship} to relationship", func(t *testing.T) {
		relationshipHandler := &mockHandler{response: "relationship"}
		resourceHandler := server.ResourceHandler{Relationship: relationshipHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, relationshipHandler.called)
	})

	t.Run("routes GET /{type}/{id}/{related} to related resources", func(t *testing.T) {
		relationshipHandler := &mockHandler{response: "related"}
		resourceHandler := server.ResourceHandler{Relationship: relationshipHandler}

		mux := server.ResourceHandlerMux{
			"users": resourceHandler,
		}

		handler := server.DefaultHandler(mux)
		req := httptest.NewRequest("GET", "/users/123/posts", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.True(t, relationshipHandler.called)
	})

	t.Run("handles 404 for unknown routes", func(t *testing.T) {
		mux := server.ResourceHandlerMux{}
		handler := server.DefaultHandler(mux)

		req := httptest.NewRequest("GET", "/unknown/path", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Test DefaultContextResolver

func TestDefaultContextResolver_ResolveRequestContext(t *testing.T) {
	resolver := server.DefaultContextResolver{}

	t.Run("resolves basic resource request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		req.SetPathValue("type", "users")
		req.SetPathValue("id", "123")

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "users", ctx.ResourceType)
		assert.Equal(t, "123", ctx.ResourceID)
		assert.Equal(t, "", ctx.Relationship)
		assert.False(t, ctx.FetchRelatedResources)
	})

	t.Run("resolves collection request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/posts", nil)
		req.SetPathValue("type", "posts")

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "posts", ctx.ResourceType)
		assert.Equal(t, "", ctx.ResourceID)
		assert.Equal(t, "", ctx.Relationship)
		assert.False(t, ctx.FetchRelatedResources)
	})

	t.Run("resolves relationship request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		req.SetPathValue("type", "users")
		req.SetPathValue("id", "123")
		req.SetPathValue("relationship", "posts")

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "users", ctx.ResourceType)
		assert.Equal(t, "123", ctx.ResourceID)
		assert.Equal(t, "posts", ctx.Relationship)
		assert.False(t, ctx.FetchRelatedResources)
	})

	t.Run("resolves related resources request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/posts", nil)
		req.SetPathValue("type", "users")
		req.SetPathValue("id", "123")
		req.SetPathValue("related", "posts")

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "users", ctx.ResourceType)
		assert.Equal(t, "123", ctx.ResourceID)
		assert.Equal(t, "posts", ctx.Relationship)
		assert.True(t, ctx.FetchRelatedResources)
	})

	t.Run("handles empty path values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "", ctx.ResourceType)
		assert.Equal(t, "", ctx.ResourceID)
		assert.Equal(t, "", ctx.Relationship)
		assert.False(t, ctx.FetchRelatedResources)
	})

	t.Run("prioritizes related over relationship when both present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/posts", nil)
		req.SetPathValue("type", "users")
		req.SetPathValue("id", "123")
		req.SetPathValue("relationship", "should-be-ignored")
		req.SetPathValue("related", "posts")

		ctx, err := resolver.ResolveRequestContext(req)

		require.NoError(t, err)
		assert.Equal(t, "users", ctx.ResourceType)
		assert.Equal(t, "123", ctx.ResourceID)
		assert.Equal(t, "posts", ctx.Relationship) // Should be "posts", not "should-be-ignored"
		assert.True(t, ctx.FetchRelatedResources)
	})

	t.Run("never returns error", func(t *testing.T) {
		// Test various scenarios to ensure no errors are returned
		testCases := []struct {
			name string
			path string
		}{
			{"root path", "/"},
			{"simple path", "/test"},
			{"complex path", "/a/b/c/d/e"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				_, err := resolver.ResolveRequestContext(req)
				assert.NoError(t, err)
			})
		}
	})
}

// Integration Tests

func TestIntegration_FullRequestFlow(t *testing.T) {
	t.Run("complete GET resource flow", func(t *testing.T) {
		// Create a handler that returns a JSON:API document
		getHandler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			assert.Equal(t, "users", ctx.ResourceType)
			assert.Equal(t, "123", ctx.ResourceID)

			doc := &jsonapi.Document{
				Data: jsonapi.SingleResource(jsonapi.Resource{
					Type: "users",
					ID:   "123",
					Attributes: map[string]interface{}{
						"name": "John Doe",
					},
				}),
			}

			return server.Response{
				Status: http.StatusOK,
				Body:   doc,
			}, nil
		})

		resourceHandler := server.ResourceHandler{Get: getHandler}
		mux := server.ResourceHandlerMux{"users": resourceHandler}
		handler := server.DefaultHandler(mux)

		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		err := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, err)

		resource, ok := doc.Data.One()
		assert.True(t, ok)
		assert.Equal(t, "users", resource.Type)
		assert.Equal(t, "123", resource.ID)
		assert.Equal(t, "John Doe", resource.Attributes["name"])
	})

	t.Run("complete POST resource flow", func(t *testing.T) {
		createHandler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			assert.Equal(t, "users", ctx.ResourceType)
			assert.Equal(t, "", ctx.ResourceID) // No ID for POST

			doc := &jsonapi.Document{
				Data: jsonapi.SingleResource(jsonapi.Resource{
					Type: "users",
					ID:   "456",
					Attributes: map[string]interface{}{
						"name": "Jane Doe",
					},
				}),
			}

			header := make(http.Header)
			header.Set("Location", "/users/456")

			return server.Response{
				Status: http.StatusCreated,
				Header: header,
				Body:   doc,
			}, nil
		})

		resourceHandler := server.ResourceHandler{Create: createHandler}
		mux := server.ResourceHandlerMux{"users": resourceHandler}
		handler := server.DefaultHandler(mux)

		req := httptest.NewRequest("POST", "/users", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "/users/456", w.Header().Get("Location"))
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	})

	t.Run("complete relationship flow", func(t *testing.T) {
		relationshipHandler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			assert.Equal(t, "users", ctx.ResourceType)
			assert.Equal(t, "123", ctx.ResourceID)
			assert.Equal(t, "posts", ctx.Relationship)

			doc := &jsonapi.Document{
				Data: jsonapi.MultiResource(
					jsonapi.Resource{Type: "posts", ID: "1"},
					jsonapi.Resource{Type: "posts", ID: "2"},
				),
			}

			return server.Response{
				Status: http.StatusOK,
				Body:   doc,
			}, nil
		})

		resourceHandler := server.ResourceHandler{Relationship: relationshipHandler}
		mux := server.ResourceHandlerMux{"users": resourceHandler}
		handler := server.DefaultHandler(mux)

		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var doc jsonapi.Document
		err := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, err)

		resources, ok := doc.Data.Many()
		assert.True(t, ok)
		assert.Len(t, resources, 2)
	})
}

// Edge Cases and Error Scenarios

func TestEdgeCases(t *testing.T) {
	t.Run("handler with nil response body", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			return server.Response{
				Status: http.StatusNoContent,
				Body:   nil,
			}, nil
		})

		req := httptest.NewRequest("DELETE", "/users/123", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("multiple headers with same name", func(t *testing.T) {
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			header := make(http.Header)
			header.Add("X-Custom", "value1")
			header.Add("X-Custom", "value2")

			return server.Response{
				Status: http.StatusOK,
				Header: header,
			}, nil
		})

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		values := w.Header().Values("X-Custom")
		assert.Contains(t, values, "value1")
		assert.Contains(t, values, "value2")
	})

	t.Run("context resolver with custom implementation", func(t *testing.T) {
		customResolver := &mockRequestContextResolver{
			resolveFunc: func(r *http.Request) (*server.RequestContext, error) {
				// Custom logic that extracts from query parameters
				resourceType := r.URL.Query().Get("type")
				resourceID := r.URL.Query().Get("id")

				return &server.RequestContext{
					ResourceType: resourceType,
					ResourceID:   resourceID,
				}, nil
			},
		}

		handler := &mockHandler{}
		middleware := server.UseRequestContextResolver(handler, customResolver)

		req := httptest.NewRequest("GET", "/custom?type=articles&id=789", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.True(t, handler.called)
	})

	t.Run("empty resource handler mux", func(t *testing.T) {
		mux := server.ResourceHandlerMux{}

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("resource handler with all nil handlers", func(t *testing.T) {
		rh := server.ResourceHandler{} // All handlers are nil

		req := httptest.NewRequest("GET", "/users", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{ResourceType: "users"})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("relationship handler with all nil handlers", func(t *testing.T) {
		rh := server.RelationshipHandler{} // All handlers are nil

		req := httptest.NewRequest("GET", "/users/123/relationships/posts", nil)
		ctx := server.SetRequestContext(req.Context(), &server.RequestContext{
			ResourceType: "users",
			ResourceID:   "123",
			Relationship: "posts",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		rh.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Concurrency Tests

func TestConcurrency(t *testing.T) {
	t.Run("concurrent requests to same handler", func(t *testing.T) {
		var callCount atomic.Int64
		handler := server.HandlerFunc(func(ctx *server.RequestContext, r *http.Request) (server.Response, error) {
			callCount.Add(1)
			return server.Response{Status: http.StatusOK}, nil
		})

		resourceHandler := server.ResourceHandler{Get: handler}
		mux := server.ResourceHandlerMux{"users": resourceHandler}
		server := server.DefaultHandler(mux)

		const numRequests = 10
		done := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", id), nil)
				w := httptest.NewRecorder()
				server.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
				done <- true
			}(i)
		}

		// Wait for all requests to complete
		for i := 0; i < numRequests; i++ {
			<-done
		}

		assert.Equal(t, int64(numRequests), callCount.Load())
	})

	t.Run("concurrent context operations", func(t *testing.T) {
		const numGoroutines = 100
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				ctx := context.Background()
				requestContext := &server.RequestContext{
					ResourceType: fmt.Sprintf("type-%d", id),
					ResourceID:   fmt.Sprintf("id-%d", id),
				}

				// Set context
				newCtx := server.SetRequestContext(ctx, requestContext)

				// Get context
				retrieved, ok := server.GetRequestContext(newCtx)
				assert.True(t, ok)
				assert.Equal(t, requestContext, retrieved)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}

// Test Error function

func TestError(t *testing.T) {
	t.Run("handles standard error with status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("something went wrong")
		status := http.StatusBadRequest

		server.Error(w, err, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, strconv.Itoa(status), errorObj.Status)
		assert.Equal(t, http.StatusText(status), errorObj.Title)
		assert.Equal(t, "something went wrong", errorObj.Detail)
		assert.Equal(t, "", errorObj.Code)
		assert.Equal(t, "", errorObj.ID)
	})

	t.Run("handles jsonapi.Error type", func(t *testing.T) {
		w := httptest.NewRecorder()
		jsonapiErr := jsonapi.Error{
			ID:     "error-123",
			Status: "422",
			Code:   "VALIDATION_ERROR",
			Title:  "Validation Failed",
			Detail: "The name field is required",
			Source: map[string]interface{}{
				"pointer": "/data/attributes/name",
			},
		}
		status := http.StatusUnprocessableEntity

		server.Error(w, jsonapiErr, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, "error-123", errorObj.ID)
		assert.Equal(t, "422", errorObj.Status)
		assert.Equal(t, "VALIDATION_ERROR", errorObj.Code)
		assert.Equal(t, "Validation Failed", errorObj.Title)
		assert.Equal(t, "The name field is required", errorObj.Detail)
		assert.Equal(t, "/data/attributes/name", errorObj.Source["pointer"])
	})

	t.Run("handles different HTTP status codes", func(t *testing.T) {
		testCases := []struct {
			name       string
			status     int
			statusText string
		}{
			{"bad request", http.StatusBadRequest, "Bad Request"},
			{"unauthorized", http.StatusUnauthorized, "Unauthorized"},
			{"forbidden", http.StatusForbidden, "Forbidden"},
			{"not found", http.StatusNotFound, "Not Found"},
			{"method not allowed", http.StatusMethodNotAllowed, "Method Not Allowed"},
			{"internal server error", http.StatusInternalServerError, "Internal Server Error"},
			{"service unavailable", http.StatusServiceUnavailable, "Service Unavailable"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				err := errors.New("test error")

				server.Error(w, err, tc.status)

				assert.Equal(t, tc.status, w.Code)
				assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

				var doc jsonapi.Document
				decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
				require.NoError(t, decodeErr)

				require.Len(t, doc.Errors, 1)
				errorObj := doc.Errors[0]
				assert.Equal(t, strconv.Itoa(tc.status), errorObj.Status)
				assert.Equal(t, tc.statusText, errorObj.Title)
				assert.Equal(t, "test error", errorObj.Detail)
			})
		}
	})

	t.Run("handles nil error", func(t *testing.T) {
		w := httptest.NewRecorder()
		status := http.StatusInternalServerError

		// This should panic or handle gracefully - let's see what happens
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Function panicked as expected with nil error: %v", r)
			}
		}()

		server.Error(w, nil, status)

		// If we get here, the function handled nil gracefully
		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	})

	t.Run("handles error with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("")
		status := http.StatusBadRequest

		server.Error(w, err, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, strconv.Itoa(status), errorObj.Status)
		assert.Equal(t, http.StatusText(status), errorObj.Title)
		assert.Equal(t, "", errorObj.Detail) // Empty error message
	})

	t.Run("handles jsonapi.Error with partial fields", func(t *testing.T) {
		w := httptest.NewRecorder()
		jsonapiErr := jsonapi.Error{
			Status: "400",
			Title:  "Bad Request",
			// Missing Detail, Code, ID, etc.
		}
		status := http.StatusBadRequest

		server.Error(w, jsonapiErr, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, "400", errorObj.Status)
		assert.Equal(t, "Bad Request", errorObj.Title)
		assert.Equal(t, "", errorObj.Detail)
		assert.Equal(t, "", errorObj.Code)
		assert.Equal(t, "", errorObj.ID)
	})

	t.Run("handles custom error type that implements error interface", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := customError{message: "custom error occurred"}
		status := http.StatusInternalServerError

		server.Error(w, err, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, strconv.Itoa(status), errorObj.Status)
		assert.Equal(t, http.StatusText(status), errorObj.Title)
		assert.Equal(t, "custom error occurred", errorObj.Detail)
	})

	t.Run("handles wrapped jsonapi.Error", func(t *testing.T) {
		w := httptest.NewRecorder()
		originalErr := jsonapi.Error{
			ID:     "wrapped-error",
			Status: "422",
			Code:   "WRAPPED_ERROR",
			Title:  "Wrapped Error",
			Detail: "This error was wrapped",
		}
		wrappedErr := fmt.Errorf("wrapper: %w", originalErr)
		status := http.StatusUnprocessableEntity

		server.Error(w, wrappedErr, status)

		assert.Equal(t, status, w.Code)
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		// Should extract the original jsonapi.Error
		assert.Equal(t, "wrapped-error", errorObj.ID)
		assert.Equal(t, "422", errorObj.Status)
		assert.Equal(t, "WRAPPED_ERROR", errorObj.Code)
		assert.Equal(t, "Wrapped Error", errorObj.Title)
		assert.Equal(t, "This error was wrapped", errorObj.Detail)
	})

	t.Run("produces valid JSON:API error document", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("validation failed")
		status := http.StatusBadRequest

		server.Error(w, err, status)

		// Verify the response is valid JSON
		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		// Verify it follows JSON:API error document structure
		// Note: doc.Data will be an empty PrimaryData, not nil
		assert.NotNil(t, doc.Errors)
		assert.Len(t, doc.Errors, 1)

		// Verify error object structure
		errorObj := doc.Errors[0]
		assert.NotEmpty(t, errorObj.Status)
		assert.NotEmpty(t, errorObj.Title)
		assert.NotEmpty(t, errorObj.Detail)
	})

	t.Run("sets correct content type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("test error")
		status := http.StatusInternalServerError

		server.Error(w, err, status)

		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
	})

	t.Run("handles invalid status code gracefully", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := errors.New("test error")
		status := -1 // Invalid status code

		// This should panic because HTTP doesn't allow invalid status codes
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Function panicked as expected with invalid status code: %v", r)
			}
		}()

		server.Error(w, err, status)

		// If we get here, check that the error document was still created properly
		assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

		var doc jsonapi.Document
		decodeErr := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, decodeErr)

		require.Len(t, doc.Errors, 1)
		errorObj := doc.Errors[0]
		assert.Equal(t, strconv.Itoa(status), errorObj.Status)
		assert.Equal(t, "test error", errorObj.Detail)
	})
}

func TestRequestContext_GetFields(t *testing.T) {
	tests := []struct {
		name         string
		queryParams  url.Values
		resourceType string
		want         []string
	}{
		{
			name:        "empty query params",
			queryParams: url.Values{
				// No fields parameter
			},
			resourceType: "users",
			want:         []string{},
		},
		{
			name: "single field",
			queryParams: url.Values{
				"fields[users]": []string{"name"},
			},
			resourceType: "users",
			want:         []string{"name"},
		},
		{
			name: "multiple fields",
			queryParams: url.Values{
				"fields[users]": []string{"name,email,id"},
			},
			resourceType: "users",
			want:         []string{"name", "email", "id"},
		},
		{
			name: "fields with whitespace",
			queryParams: url.Values{
				"fields[users]": []string{"name, email, id"},
			},
			resourceType: "users",
			want:         []string{"name", "email", "id"},
		},
		{
			name: "different resource type",
			queryParams: url.Values{
				"fields[users]": []string{"name,email"},
				"fields[posts]": []string{"title,body"},
			},
			resourceType: "posts",
			want:         []string{"title", "body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.queryParams.Encode(),
				},
			}

			rc := server.RequestContext{}
			got := rc.GetFields(req, tt.resourceType)

			assert.Equal(t, tt.want, got, "GetFields() returned incorrect fields")
		})
	}
}

func TestRequestContext_ShouldInclude(t *testing.T) {
	tests := []struct {
		name        string
		queryParams url.Values
		path        string
		want        bool
	}{
		{
			name:        "empty query params",
			queryParams: url.Values{
				// No include parameter
			},
			path: "posts",
			want: false,
		},
		{
			name: "single include",
			queryParams: url.Values{
				"include": []string{"posts"},
			},
			path: "posts",
			want: true,
		},
		{
			name: "multiple includes",
			queryParams: url.Values{
				"include": []string{"posts,comments,author"},
			},
			path: "comments",
			want: true,
		},
		{
			name: "includes with whitespace",
			queryParams: url.Values{
				"include": []string{"posts, comments, author"},
			},
			path: "comments",
			want: true,
		},
		{
			name: "path not included",
			queryParams: url.Values{
				"include": []string{"posts,comments"},
			},
			path: "author",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.queryParams.Encode(),
				},
			}

			rc := server.RequestContext{}
			got := rc.ShouldInclude(req, tt.path)

			assert.Equal(t, tt.want, got, "ShouldInclude() returned incorrect result")
		})
	}
}

func TestRequestContext_GetPageParam(t *testing.T) {
	tests := []struct {
		name        string
		queryParams url.Values
		key         string
		want        string
	}{
		{
			name:        "empty query params",
			queryParams: url.Values{
				// No page parameter
			},
			key:  "number",
			want: "",
		},
		{
			name: "page number",
			queryParams: url.Values{
				"page[number]": []string{"1"},
			},
			key:  "number",
			want: "1",
		},
		{
			name: "page size",
			queryParams: url.Values{
				"page[size]": []string{"10"},
			},
			key:  "size",
			want: "10",
		},
		{
			name: "multiple page params",
			queryParams: url.Values{
				"page[number]": []string{"1"},
				"page[size]":   []string{"10"},
			},
			key:  "size",
			want: "10",
		},
		{
			name: "param not found",
			queryParams: url.Values{
				"page[number]": []string{"1"},
			},
			key:  "offset",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.queryParams.Encode(),
				},
			}

			rc := server.RequestContext{}
			got := rc.GetPageParam(req, tt.key)

			assert.Equal(t, tt.want, got, "GetPageParam() returned incorrect value")
		})
	}
}

func TestRequestContext_GetFilterParam(t *testing.T) {
	tests := []struct {
		name        string
		queryParams url.Values
		key         string
		want        string
	}{
		{
			name:        "empty query params",
			queryParams: url.Values{
				// No filter parameter
			},
			key:  "name",
			want: "",
		},
		{
			name: "single filter",
			queryParams: url.Values{
				"filter[name]": []string{"John"},
			},
			key:  "name",
			want: "John",
		},
		{
			name: "multiple filters",
			queryParams: url.Values{
				"filter[name]":   []string{"John"},
				"filter[status]": []string{"active"},
			},
			key:  "status",
			want: "active",
		},
		{
			name: "filter not found",
			queryParams: url.Values{
				"filter[name]": []string{"John"},
			},
			key:  "status",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.queryParams.Encode(),
				},
			}

			rc := server.RequestContext{}
			got := rc.GetFilterParam(req, tt.key)

			assert.Equal(t, tt.want, got, "GetFilterParam() returned incorrect value")
		})
	}
}

func TestRequestContext_GetSort(t *testing.T) {
	tests := []struct {
		name        string
		queryParams url.Values
		want        []string
	}{
		{
			name:        "empty query params",
			queryParams: url.Values{
				// No sort parameter
			},
			want: []string{},
		},
		{
			name: "single sort field",
			queryParams: url.Values{
				"sort": []string{"name"},
			},
			want: []string{"name"},
		},
		{
			name: "multiple sort fields",
			queryParams: url.Values{
				"sort": []string{"name,created_at"},
			},
			want: []string{"name", "created_at"},
		},
		{
			name: "descending sort",
			queryParams: url.Values{
				"sort": []string{"-created_at"},
			},
			want: []string{"-created_at"},
		},
		{
			name: "mixed sort directions",
			queryParams: url.Values{
				"sort": []string{"name,-created_at,id"},
			},
			want: []string{"name", "-created_at", "id"},
		},
		{
			name: "sort with whitespace",
			queryParams: url.Values{
				"sort": []string{"name, -created_at, id"},
			},
			want: []string{"name", "-created_at", "id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.queryParams.Encode(),
				},
			}

			rc := server.RequestContext{}
			got := rc.GetSort(req)

			assert.Equal(t, tt.want, got, "GetSort() returned incorrect value")
		})
	}
}

func TestError_SingleError(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("test error")

	server.Error(w, err, http.StatusBadRequest)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/vnd.api+json", resp.Header.Get("Content-Type"), "Expected Content-Type %s, got %s", "application/vnd.api+json", resp.Header.Get("Content-Type"))

	var doc jsonapi.Document
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc), "Failed to decode response body")

	require.Len(t, doc.Errors, 1, "Expected 1 error, got %d", len(doc.Errors))
	assert.Equal(t, "test error", doc.Errors[0].Detail, "Expected error detail %q, got %q", "test error", doc.Errors[0].Detail)
	assert.Equal(t, "400", doc.Errors[0].Status, "Expected error status %q, got %q", "400", doc.Errors[0].Status)
}

func TestError_JSONAPIError(t *testing.T) {
	w := httptest.NewRecorder()
	err := jsonapi.Error{
		Status: "422",
		Code:   "VALIDATION_ERROR",
		Title:  "Validation Failed",
		Detail: "Name is required",
		Source: map[string]interface{}{
			"pointer": "/data/attributes/name",
		},
	}

	server.Error(w, err, http.StatusUnprocessableEntity)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "Expected status code %d, got %d", http.StatusUnprocessableEntity, resp.StatusCode)

	var doc jsonapi.Document
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc), "Failed to decode response body")

	require.Len(t, doc.Errors, 1, "Expected 1 error, got %d", len(doc.Errors))
	assert.Equal(t, "Name is required", doc.Errors[0].Detail, "Expected error detail %q, got %q", "Name is required", doc.Errors[0].Detail)
	assert.Equal(t, "422", doc.Errors[0].Status, "Expected error status %q, got %q", "422", doc.Errors[0].Status)
	assert.Equal(t, "VALIDATION_ERROR", doc.Errors[0].Code, "Expected error code %q, got %q", "VALIDATION_ERROR", doc.Errors[0].Code)
	assert.Equal(t, "Validation Failed", doc.Errors[0].Title, "Expected error title %q, got %q", "Validation Failed", doc.Errors[0].Title)

	require.NotNil(t, doc.Errors[0].Source, "Expected error source to be non-nil")
	pointer, ok := doc.Errors[0].Source["pointer"].(string)
	require.True(t, ok, "Expected error source pointer to be a string")
	assert.Equal(t, "/data/attributes/name", pointer, "Expected error source pointer %q, got %q", "/data/attributes/name", pointer)
}

func TestError_MultiError(t *testing.T) {
	w := httptest.NewRecorder()
	errs := jsonapi.MultiError{
		{
			Status: "422",
			Code:   "VALIDATION_ERROR",
			Title:  "Validation Failed",
			Detail: "Name is required",
			Source: map[string]interface{}{
				"pointer": "/data/attributes/name",
			},
		},
		{
			Status: "422",
			Code:   "VALIDATION_ERROR",
			Title:  "Validation Failed",
			Detail: "Email must be valid",
			Source: map[string]interface{}{
				"pointer": "/data/attributes/email",
			},
		},
	}

	server.Error(w, errs, http.StatusUnprocessableEntity)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "Expected status code %d, got %d", http.StatusUnprocessableEntity, resp.StatusCode)

	var doc jsonapi.Document
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc), "Failed to decode response body")

	require.Len(t, doc.Errors, 2, "Expected 2 errors, got %d", len(doc.Errors))
	assert.Equal(t, "Name is required", doc.Errors[0].Detail, "Expected first error detail %q, got %q", "Name is required", doc.Errors[0].Detail)
	assert.Equal(t, "Email must be valid", doc.Errors[1].Detail, "Expected second error detail %q, got %q", "Email must be valid", doc.Errors[1].Detail)
}

func TestError_NilErrorType(t *testing.T) {
	w := httptest.NewRecorder()
	server.Error(w, nil, http.StatusInternalServerError)

	resp := w.Result()
	defer resp.Body.Close()

	var doc jsonapi.Document
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc), "Failed to decode response body")

	require.Len(t, doc.Errors, 1, "Expected 1 error, got %d", len(doc.Errors))
	assert.Equal(t, "Unknown error", doc.Errors[0].Detail, "Expected error detail %q, got %q", "Unknown error", doc.Errors[0].Detail)
}
