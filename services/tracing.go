package services

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	tracesExporterOTLP = "otlp"
	tracesExporterNone = "none"

	defaultServiceName = "whatsmiau"
)

var tracer = otel.Tracer("github.com/verbeux-ai/whatsmiau/services")

func StartTracing(ctx context.Context) (func(context.Context) error, error) {
	switch env.Env.OtelTracesExporter {
	case tracesExporterNone:
		return func(context.Context) error { return nil }, nil
	case tracesExporterOTLP:
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_EXPORTER %q: use %q or %q",
			env.Env.OtelTracesExporter, tracesExporterOTLP, tracesExporterNone)
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	res, err := newTraceResource(ctx)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		zap.L().Warn("opentelemetry error", zap.Error(err))
	}))

	return provider.Shutdown, nil
}

func newTraceResource(ctx context.Context) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(defaultServiceName)),
		resource.WithFromEnv(),
	)
	if errors.Is(err, resource.ErrPartialResource) {
		zap.L().Warn("ignoring invalid OTEL_RESOURCE_ATTRIBUTES entries", zap.Error(err))
		return res, nil
	}
	if err != nil {
		return nil, fmt.Errorf("build trace resource: %w", err)
	}
	return res, nil
}

type redisTracingHook struct {
	attributes []attribute.KeyValue
}

func newRedisTracingHook(addr string) redisTracingHook {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return redisTracingHook{attributes: []attribute.KeyValue{
		semconv.DBSystemNameRedis,
		semconv.ServerAddress(host),
	}}
}

func (h redisTracingHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if !trace.SpanFromContext(ctx).IsRecording() {
		return ctx, nil
	}
	ctx, _ = tracer.Start(ctx, "redis."+cmd.Name(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(h.attributes...),
		trace.WithAttributes(semconv.DBOperationName(cmd.Name())),
	)
	return ctx, nil
}

func (redisTracingHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	span := trace.SpanFromContext(ctx)
	if err := cmd.Err(); err != nil && !errors.Is(err, redis.Nil) {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	return nil
}

func (redisTracingHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (redisTracingHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}
