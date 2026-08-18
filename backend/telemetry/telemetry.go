// Package telemetry wires up OpenTelemetry tracing and OTel-bridged
// logging for the server, exporting both via OTLP/gRPC.
package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Setup configures the global TracerProvider/propagator and returns a
// slog.Handler for serviceName, both exporting via OTLP/gRPC to
// OTEL_EXPORTER_OTLP_ENDPOINT. If that env var is unset — e.g. `go run
// ./cmd/server` outside docker-compose — telemetry export is skipped
// entirely: tracer.Start calls fall through to the global no-op
// TracerProvider, and the returned handler is a plain stdout JSON
// handler, rather than the server blocking on or erroring against a
// collector that isn't running.
//
// When export is enabled, the returned handler still always writes
// stdout JSON (so `docker compose logs`/local runs are unaffected) and
// additionally fans out every record to the OTel Logs pipeline via the
// otelslog bridge, which stamps each record with the active trace/span
// ID for correlation in Grafana.
//
// Callers must invoke the returned shutdown func to flush buffered spans
// and log records before the process exits.
func Setup(ctx context.Context, serviceName string) (handler slog.Handler, shutdown func(context.Context) error, err error) {
	stdout := slog.NewJSONHandler(os.Stdout, nil)
	noop := func(context.Context) error { return nil }

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return stdout, noop, nil
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	fanout := fanoutHandler{
		stdout: stdout,
		otel:   otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)),
	}

	shutdown = func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		return lp.Shutdown(ctx)
	}
	return fanout, shutdown, nil
}

// fanoutHandler forwards every log record to both a local stdout handler
// and the OTel Logs bridge.
type fanoutHandler struct {
	stdout slog.Handler
	otel   slog.Handler
}

func (f fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return f.stdout.Enabled(ctx, level) || f.otel.Enabled(ctx, level)
}

func (f fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	if err := f.stdout.Handle(ctx, record.Clone()); err != nil {
		return err
	}
	return f.otel.Handle(ctx, record.Clone())
}

func (f fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return fanoutHandler{stdout: f.stdout.WithAttrs(attrs), otel: f.otel.WithAttrs(attrs)}
}

func (f fanoutHandler) WithGroup(name string) slog.Handler {
	return fanoutHandler{stdout: f.stdout.WithGroup(name), otel: f.otel.WithGroup(name)}
}
