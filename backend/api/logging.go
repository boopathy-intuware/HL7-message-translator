package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// requestFields accumulates fields a handler wants included in the
// structured log line RequestLogger emits after the handler returns.
// message_type and parse_status are only meaningful for the HL7 ingest
// endpoint, so they're populated via setRequestFields rather than parsed
// generically from every request/response.
type requestFields struct {
	messageType string
	parseStatus string
}

type requestFieldsKey struct{}

// setRequestFields records message_type/parse_status on the request's
// context for RequestLogger to pick up. It's a no-op if the context wasn't
// produced by RequestLogger (e.g. in a handler unit test that doesn't wire
// the middleware) — callers don't need to guard against that themselves.
func setRequestFields(ctx context.Context, messageType, parseStatus string) {
	if f, ok := ctx.Value(requestFieldsKey{}).(*requestFields); ok {
		f.messageType = messageType
		f.parseStatus = parseStatus
	}
}

// RequestLogger returns middleware that logs one structured JSON line per
// request via logger: method, path, status, and latency for every request,
// plus message_type/parse_status whenever the handler populates them via
// setRequestFields.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fields := &requestFields{}
			ctx := context.WithValue(r.Context(), requestFieldsKey{}, fields)
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r.WithContext(ctx))

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"latency_ms", time.Since(start).Milliseconds(),
			}
			if fields.messageType != "" {
				attrs = append(attrs, "message_type", fields.messageType)
			}
			if fields.parseStatus != "" {
				attrs = append(attrs, "parse_status", fields.parseStatus)
			}
			logger.Info("http_request", attrs...)
		})
	}
}

// statusWriter captures the status code written by the wrapped handler so
// RequestLogger can include it in the post-request log line.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
