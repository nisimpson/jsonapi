package jsonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiddlewareFunc_Use(t *testing.T) {
	called := false
	middleware := MiddlewareFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		called = true
		w.Header().Set("X-Test", "middleware")
		next.ServeHTTP(w, r)
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Use(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, "middleware", w.Header().Get("X-Test"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUse_SingleMiddleware(t *testing.T) {
	middleware := MiddlewareFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		w.Header().Set("X-Middleware", "1")
		next.ServeHTTP(w, r)
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := Use(handler, middleware)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, "1", w.Header().Get("X-Middleware"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUse_MultipleMiddleware(t *testing.T) {
	var order []string

	middleware1 := MiddlewareFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		order = append(order, "middleware1-before")
		next.ServeHTTP(w, r)
		order = append(order, "middleware1-after")
	})

	middleware2 := MiddlewareFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		order = append(order, "middleware2-before")
		next.ServeHTTP(w, r)
		order = append(order, "middleware2-after")
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := Use(handler, middleware1, middleware2)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"handler",
		"middleware2-after",
		"middleware1-after",
	}
	assert.Equal(t, expected, order)
}

func TestUse_NoMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := Use(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUseRequestResolver(t *testing.T) {
	resolver := DefaultRequestResolver{}
	middleware := UseRequestResolver(resolver)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := FromContext(r.Context())
		assert.NotNil(t, request)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Use(handler)

	req := httptest.NewRequest("GET", "/users/123", nil)
	req.SetPathValue("type", "users")
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

type testMiddleware struct {
	called bool
}

func (m *testMiddleware) Use(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.called = true
		next.ServeHTTP(w, r)
	})
}

func TestMiddleware_Interface(t *testing.T) {
	middleware := &testMiddleware{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Use(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.True(t, middleware.called)
	assert.Equal(t, http.StatusOK, w.Code)
}
