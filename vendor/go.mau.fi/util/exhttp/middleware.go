package exhttp

import "net/http"

// Middleware represents a middleware that can be applied to an [http.Handler].
type Middleware func(http.Handler) http.Handler

// ApplyMiddleware applies the provided [Middleware] functions to the given
// router. The middlewares will be applied in the order they are provided.
func ApplyMiddleware(router http.Handler, middlewares ...Middleware) http.Handler {
	// Apply middlewares in reverse order because the first middleware provided
	// needs to be the outermost one.
	for i := len(middlewares) - 1; i >= 0; i-- {
		router = middlewares[i](router)
	}
	return router
}

// StripPrefix is a wrapper for [http.StripPrefix] is compatible with the middleware pattern.
func StripPrefix(prefix string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.StripPrefix(prefix, next)
	}
}
