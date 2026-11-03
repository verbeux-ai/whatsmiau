package whatsmiau

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
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

func startParentSpan(t *testing.T) (context.Context, trace.Span) {
	t.Helper()
	ctx, span := otel.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { span.End() })
	return ctx, span
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func doTracedRequest(t *testing.T, ctx context.Context, transport http.RoundTripper, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
}

func TestTracingTransportSkipsSpanWithoutParent(t *testing.T) {
	recorder := recordSpans(t)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	doTracedRequest(t, context.Background(), tracingTransport{base: http.DefaultTransport}, server.URL)

	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans without a parent, got %d", len(spans))
	}
}

func TestTracingTransportEndsClientSpanWhenBodyIsClosed(t *testing.T) {
	recorder := recordSpans(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	ctx, parent := startParentSpan(t)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/media?signature=secret", nil)
	res, err := tracingTransport{operation: "media.download", base: http.DefaultTransport}.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected the span to stay open until the body is closed, got %d ended spans", len(spans))
	}
	_ = res.Body.Close()

	spans := recorder.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != "media.download" || span.SpanKind != trace.SpanKindClient {
		t.Fatalf("unexpected span name/kind: %q %v", span.Name, span.SpanKind)
	}
	if span.Parent.SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("expected span to be a child of the request span")
	}
	attributes := spanAttributes(span)
	wantAttributes := map[string]string{
		string(semconv.HTTPRequestMethodKey):      http.MethodGet,
		string(semconv.ServerAddressKey):          "127.0.0.1",
		string(semconv.HTTPResponseStatusCodeKey): "200",
	}
	for key, want := range wantAttributes {
		if attributes[key] != want {
			t.Fatalf("attribute %s = %q, want %q", key, attributes[key], want)
		}
	}
	for key, value := range attributes {
		if strings.Contains(value, "signature=secret") {
			t.Fatalf("attribute %s leaks the request URL: %q", key, value)
		}
	}
}

func spanAttributes(span tracetest.SpanStub) map[string]string {
	attributes := make(map[string]string, len(span.Attributes))
	for _, attr := range span.Attributes {
		attributes[string(attr.Key)] = attr.Value.String()
	}
	return attributes
}

func TestChildSpansCarryTheRequestInstanceID(t *testing.T) {
	recorder := recordSpans(t)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	ctx, _ := startParentSpan(t)

	doTracedRequest(t, ContextWithInstanceID(ctx, "inst-1"), tracingTransport{base: http.DefaultTransport}, server.URL)

	spans := recorder.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if got := spanAttributes(spans[0])[string(InstanceIDKey)]; got != "inst-1" {
		t.Fatalf("instance id attribute = %q, want %q", got, "inst-1")
	}
}

func TestTracingTransportMarksErrorResponsesAsFailed(t *testing.T) {
	recorder := recordSpans(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, _ := startParentSpan(t)

	doTracedRequest(t, ctx, tracingTransport{base: http.DefaultTransport}, server.URL)

	spans := recorder.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("expected one span with error status, got %+v", spans)
	}
}

func TestTracingTransportMarksTransportErrorsAsFailed(t *testing.T) {
	recorder := recordSpans(t)
	ctx, _ := startParentSpan(t)
	failing := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid", nil)
	if _, err := (tracingTransport{base: failing}).RoundTrip(req); err == nil {
		t.Fatalf("expected the transport error to be returned")
	}

	spans := recorder.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("expected one span with error status, got %+v", spans)
	}
}

func TestTracingTransportDoesNotPropagateTraceToThirdParties(t *testing.T) {
	recordSpans(t)
	var receivedTraceParent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceParent = r.Header.Get("traceparent")
	}))
	defer server.Close()
	ctx, _ := startParentSpan(t)

	doTracedRequest(t, ctx, tracingTransport{base: http.DefaultTransport}, server.URL)

	if receivedTraceParent != "" {
		t.Fatalf("expected no traceparent sent to third parties, got %q", receivedTraceParent)
	}
}

func TestSendTimingAttributesReportsMeasuredStagesInMilliseconds(t *testing.T) {
	attributes := sendTimingAttributes(whatsmeow.MessageDebugTimings{
		PeerEncrypt: 1500 * time.Microsecond,
		Send:        20 * time.Millisecond,
	})

	want := []attribute.KeyValue{
		attribute.Float64("whatsapp.send.timing.peer_encrypt_ms", 1.5),
		attribute.Float64("whatsapp.send.timing.send_ms", 20),
	}
	if len(attributes) != len(want) {
		t.Fatalf("expected only the measured stages, got %v", attributes)
	}
	for i := range want {
		if attributes[i] != want[i] {
			t.Errorf("attribute %d = %v, want %v", i, attributes[i], want[i])
		}
	}
}

func TestFanOutSendsKeepTheTraceButStopRecordingStoreSpans(t *testing.T) {
	recordSpans(t)
	ctx, parent := startParentSpan(t)

	for _, to := range []types.JID{
		types.NewJID("120363000000000000", types.GroupServer),
		types.StatusBroadcastJID,
	} {
		sendCtx := skipStoreSpansOnFanOut(ctx, to)

		span := trace.SpanFromContext(sendCtx)
		if span.IsRecording() {
			t.Errorf("send to %s still records child spans", to)
		}
		if span.SpanContext().TraceID() != parent.SpanContext().TraceID() {
			t.Errorf("send to %s lost the trace id", to)
		}
	}
}

func TestDirectSendsKeepRecordingStoreSpans(t *testing.T) {
	recordSpans(t)
	ctx, _ := startParentSpan(t)

	sendCtx := skipStoreSpansOnFanOut(ctx, types.NewJID("5511999999999", types.DefaultUserServer))

	if !trace.SpanFromContext(sendCtx).IsRecording() {
		t.Error("direct send stopped recording child spans")
	}
}
