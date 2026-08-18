package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

// RouteTagger renames the current span — started by the otelhttp
// middleware that must wrap this one — from "<method> <raw-path>" to
// "<method> <chi-route-pattern>" once chi has matched the request to a
// route, e.g. "GET /api/messages/{id}" instead of one distinct span name
// per message ID. It must run after otelhttp's middleware (so the span
// is still open when it reads the route) and before chi dispatches to the
// matched handler completes routing, though in practice it only needs to
// run somewhere in the chain outside the router itself — chi mutates a
// single RouteContext value in place as it matches, so it's readable
// immediately after next.ServeHTTP returns regardless of nesting order.
func RouteTagger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			trace.SpanFromContext(r.Context()).SetName(r.Method + " " + rc.RoutePattern())
		}
	})
}
