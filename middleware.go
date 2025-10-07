package jsonapi

import "net/http"

// Middleware defines the interface for HTTP middleware components that can wrap handlers
// to provide cross-cutting functionality such as authentication, logging, or request processing.
type Middleware interface {
	// Use wraps the provided handler with middleware functionality.
	Use(next http.Handler) http.Handler
}

// Use applies multiple [Middleware] components to a base handler in the order provided.
// Middleware is applied in reverse order, so the first middleware in the slice
// will be the outermost wrapper and execute first in the request chain.
func Use(base http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		base = middleware[i].Use(base)
	}
	return base
}

// MiddlewareFunc is a function type that implements the [Middleware] interface.
// It provides a convenient way to create middleware from functions that follow
// the pattern of receiving a response writer, request, and next handler.
type MiddlewareFunc func(w http.ResponseWriter, r *http.Request, next http.Handler)

// Use implements the [Middleware] interface for [MiddlewareFunc].
// It wraps the function in an http.Handler that calls the middleware function
// with the appropriate parameters.
func (f MiddlewareFunc) Use(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(w, r, next)
	})
}

// UseRequestResolver creates HTTP [Middleware] that parses incoming requests using the provided resolver
// and adds the parsed information to the request context for downstream handlers.
// This middleware is essential for JSON:API request processing as it extracts resource information
// from URLs and makes it available to handlers through the request context.
func UseRequestResolver(resolver RequestResolver) Middleware {
	return MiddlewareFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		request := resolver.ResolveJSONAPIRequest(r)
		ctx := WithContext(r.Context(), request)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
