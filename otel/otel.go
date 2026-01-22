// Package otel provides optional OpenTelemetry instrumentation for gitstreams.
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	serviceName = "gitstreams"
)

// Setup initializes OpenTelemetry if OTEL environment variables are configured.
// Returns a tracer provider and a cleanup function. If OTEL is not configured,
// returns a no-op tracer and a no-op cleanup function.
//
// The cleanup function should be called before the application exits to ensure
// all spans are exported.
//
// Environment variables:
//   - OTEL_EXPORTER_OTLP_ENDPOINT: The OTLP endpoint (e.g., http://localhost:4318)
//   - OTEL_SERVICE_NAME: Optional service name (defaults to "gitstreams")
func Setup(ctx context.Context, logger *slog.Logger) (trace.TracerProvider, func() error, error) {
	// Use default logger if none provided
	if logger == nil {
		logger = slog.Default()
	}

	// Check if OTEL is configured
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Debug("OpenTelemetry not configured (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
		return noop.NewTracerProvider(), func() error { return nil }, nil
	}

	logger.Info("Initializing OpenTelemetry",
		"endpoint", endpoint,
		"service", serviceName,
	)

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(getServiceName()),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating resource: %w", err)
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // For local development
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Return cleanup function
	cleanup := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down tracer provider: %w", err)
		}
		logger.Debug("OpenTelemetry tracer provider shut down")
		return nil
	}

	logger.Info("OpenTelemetry initialized successfully")
	return tp, cleanup, nil
}

// getServiceName returns the service name from environment or default.
func getServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return serviceName
}

// Tracer returns a tracer for the gitstreams package.
func Tracer() trace.Tracer {
	return otel.Tracer(serviceName)
}
