package services

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	spanRecorderOnce sync.Once
	spanRecorder     *tracetest.InMemoryExporter
)

func recordSpans(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	spanRecorderOnce.Do(func() {
		spanRecorder = tracetest.NewInMemoryExporter()
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanRecorder)))
	})
	spanRecorder.Reset()
	return spanRecorder
}

func newTracedRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	client.AddHook(newRedisTracingHook(server.Addr()))
	t.Cleanup(func() { _ = client.Close() })
	return client, server
}

func startParentSpan(t *testing.T) context.Context {
	t.Helper()
	ctx, span := otel.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { span.End() })
	return ctx
}

func hasAttribute(attributes []attribute.KeyValue, want attribute.KeyValue) bool {
	for _, attr := range attributes {
		if attr == want {
			return true
		}
	}
	return false
}

func TestRedisTracingHookSkipsSpanWithoutParent(t *testing.T) {
	recorder := recordSpans(t)
	client, _ := newTracedRedis(t)

	if err := client.Set(context.Background(), "key", "value", 0).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}

	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans without a parent, got %d", len(spans))
	}
}

func TestRedisTracingHookCreatesChildSpanPerCommand(t *testing.T) {
	recorder := recordSpans(t)
	client, _ := newTracedRedis(t)
	ctx := startParentSpan(t)
	parent := trace.SpanContextFromContext(ctx)

	if err := client.Set(ctx, "key", "value", 0).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}

	spans := recorder.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != "redis.set" {
		t.Errorf("span name = %q, want redis.set", span.Name)
	}
	if span.SpanKind != trace.SpanKindClient {
		t.Errorf("span kind = %v, want client", span.SpanKind)
	}
	if span.Parent.SpanID() != parent.SpanID() {
		t.Error("redis span is not a child of the caller span")
	}
	for _, want := range []attribute.KeyValue{
		semconv.DBSystemNameRedis,
		semconv.DBOperationName("set"),
		semconv.ServerAddress("127.0.0.1"),
	} {
		if !hasAttribute(span.Attributes, want) {
			t.Errorf("missing attribute %s=%s", want.Key, want.Value.String())
		}
	}
}

func TestRedisTracingHookDoesNotRecordCommandArguments(t *testing.T) {
	recorder := recordSpans(t)
	client, _ := newTracedRedis(t)
	const secret = "webhook-auth-token"

	if err := client.Set(startParentSpan(t), "key", secret, 0).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}

	for _, attr := range recorder.GetSpans()[0].Attributes {
		if attr.Value.String() == secret {
			t.Fatalf("attribute %s leaked the command value", attr.Key)
		}
	}
}

func TestRedisTracingHookTreatsMissingKeyAsSuccess(t *testing.T) {
	recorder := recordSpans(t)
	client, _ := newTracedRedis(t)

	if err := client.Get(startParentSpan(t), "missing").Err(); err != redis.Nil {
		t.Fatalf("get error = %v, want redis.Nil", err)
	}

	if status := recorder.GetSpans()[0].Status.Code; status == codes.Error {
		t.Error("a cache miss was recorded as an error")
	}
}

func TestRedisTracingHookMarksFailedCommandAsError(t *testing.T) {
	recorder := recordSpans(t)
	client, server := newTracedRedis(t)
	server.SetError("boom")

	if err := client.Get(startParentSpan(t), "key").Err(); err == nil {
		t.Fatal("expected the command to fail")
	}

	if status := recorder.GetSpans()[0].Status.Code; status != codes.Error {
		t.Errorf("span status = %v, want error", status)
	}
}

func TestTraceResourceKeepsBootingWithMalformedResourceAttributes(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name,team=core")

	res, err := newTraceResource(context.Background())
	if err != nil {
		t.Fatalf("new trace resource: %v", err)
	}

	for _, want := range []attribute.KeyValue{
		semconv.ServiceName(defaultServiceName),
		attribute.String("team", "core"),
	} {
		if !hasAttribute(res.Attributes(), want) {
			t.Errorf("missing attribute %s=%s", want.Key, want.Value.String())
		}
	}
}
