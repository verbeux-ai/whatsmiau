package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	incomingTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
	incomingSpanID      = "00f067aa0ba902b7"
	incomingTraceParent = "00-" + incomingTraceID + "-" + incomingSpanID + "-01"
	pingRoute           = "/v1/instance/:instance/ping"
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
		otel.SetTextMapPropagator(propagation.TraceContext{})
	})
	spanRecorder.Reset()
	return spanRecorder
}

func serveTraced(handler echo.HandlerFunc, configure func(*http.Request)) *httptest.ResponseRecorder {
	app := echo.New()
	app.Pre(echomiddleware.Recover())
	app.Group("/v1", Tracing).GET("/instance/:instance/ping", handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/instance/inst-1/ping", nil)
	if configure != nil {
		configure(req)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func respondOK(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func onlySpan(t *testing.T, recorder *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	spans := recorder.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

func spanAttributes(span tracetest.SpanStub) map[string]string {
	attributes := make(map[string]string, len(span.Attributes))
	for _, attr := range span.Attributes {
		attributes[string(attr.Key)] = attr.Value.String()
	}
	return attributes
}

func TestTracingContinuesIncomingTraceParent(t *testing.T) {
	recorder := recordSpans(t)
	var handlerSpan trace.SpanContext

	rec := serveTraced(func(c echo.Context) error {
		handlerSpan = trace.SpanContextFromContext(c.Request().Context())
		return respondOK(c)
	}, func(req *http.Request) {
		req.Header.Set("traceparent", incomingTraceParent)
	})

	span := onlySpan(t, recorder)
	if span.SpanContext.TraceID().String() != incomingTraceID {
		t.Fatalf("trace id = %s, want %s", span.SpanContext.TraceID(), incomingTraceID)
	}
	if span.Parent.SpanID().String() != incomingSpanID || !span.Parent.IsRemote() {
		t.Fatalf("expected the caller span %s as remote parent, got %s", incomingSpanID, span.Parent.SpanID())
	}
	if handlerSpan.SpanID() != span.SpanContext.SpanID() {
		t.Fatalf("handler context does not carry the server span")
	}
	if span.Name != "GET "+pingRoute || span.SpanKind != trace.SpanKindServer {
		t.Fatalf("unexpected span name/kind: %q %v", span.Name, span.SpanKind)
	}

	attributes := spanAttributes(span)
	if attributes["http.route"] != pingRoute || attributes["whatsmiau.instance.id"] != "inst-1" || attributes["http.response.status_code"] != "200" {
		t.Fatalf("unexpected attributes: %v", attributes)
	}

	wantTraceResponse := "00-" + incomingTraceID + "-" + span.SpanContext.SpanID().String() + "-01"
	if got := rec.Header().Get(TraceResponseHeader); got != wantTraceResponse {
		t.Fatalf("traceresponse = %q, want %q", got, wantTraceResponse)
	}
}

func TestTracingReadsInstanceIDFromLifecycleRoutes(t *testing.T) {
	recorder := recordSpans(t)
	app := echo.New()
	app.Group("/v1", Tracing).GET("/instance/:id/status", respondOK)

	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/instance/inst-1/status", nil))

	if got := spanAttributes(onlySpan(t, recorder))["whatsmiau.instance.id"]; got != "inst-1" {
		t.Fatalf("instance id attribute = %q, want %q", got, "inst-1")
	}
}

func TestTracingStartsNewTraceWithoutTraceParent(t *testing.T) {
	recorder := recordSpans(t)

	rec := serveTraced(respondOK, nil)

	span := onlySpan(t, recorder)
	if span.Parent.IsValid() {
		t.Fatalf("expected a root span, got parent %s", span.Parent.SpanID())
	}
	if rec.Header().Get(TraceResponseHeader) == "" {
		t.Fatalf("expected traceresponse header on a new trace")
	}
}

func TestTracingMarksOnlyServerErrorsAsFailed(t *testing.T) {
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus string
		wantCode   codes.Code
	}{
		{
			name:       "client error returned as HTTPError",
			handler:    func(echo.Context) error { return echo.NewHTTPError(http.StatusUnauthorized) },
			wantStatus: "401",
			wantCode:   codes.Unset,
		},
		{
			name:       "plain error returned by handler",
			handler:    func(echo.Context) error { return errors.New("boom") },
			wantStatus: "500",
			wantCode:   codes.Error,
		},
		{
			name: "server error written by handler",
			handler: func(c echo.Context) error {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "down"})
			},
			wantStatus: "503",
			wantCode:   codes.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := recordSpans(t)

			serveTraced(tt.handler, nil)

			span := onlySpan(t, recorder)
			if got := spanAttributes(span)["http.response.status_code"]; got != tt.wantStatus {
				t.Fatalf("status code attribute = %s, want %s", got, tt.wantStatus)
			}
			if span.Status.Code != tt.wantCode {
				t.Fatalf("span status = %v, want %v", span.Status.Code, tt.wantCode)
			}
		})
	}
}

func TestTracingSkipsWebSocketUpgrades(t *testing.T) {
	recorder := recordSpans(t)

	rec := serveTraced(respondOK, func(req *http.Request) {
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
	})

	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans for websocket upgrades, got %d", len(spans))
	}
	if rec.Header().Get(TraceResponseHeader) != "" {
		t.Fatalf("expected no traceresponse header for websocket upgrades")
	}
}

func TestTracingMarksPanicsAsFailed(t *testing.T) {
	recorder := recordSpans(t)

	rec := serveTraced(func(echo.Context) error { panic("boom") }, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected the panic to reach Recover, got status %d", rec.Code)
	}
	span := onlySpan(t, recorder)
	if span.Status.Code != codes.Error {
		t.Fatalf("span status = %v, want error", span.Status.Code)
	}
	if got := spanAttributes(span)["http.response.status_code"]; got != "500" {
		t.Fatalf("status code attribute = %q, want 500", got)
	}
}

func TestTracingRecordsTheErrorBehindServerFailures(t *testing.T) {
	recorder := recordSpans(t)

	serveTraced(func(echo.Context) error { return errors.New("redis unavailable") }, nil)

	events := onlySpan(t, recorder).Events
	if len(events) != 1 || events[0].Name != "exception" {
		t.Fatalf("expected one exception event, got %+v", events)
	}
	for _, attr := range events[0].Attributes {
		if attr.Key == "exception.message" && attr.Value.AsString() == "redis unavailable" {
			return
		}
	}
	t.Fatalf("exception event does not carry the handler error: %+v", events[0].Attributes)
}

func TestTracingDoesNotRecordClientErrors(t *testing.T) {
	recorder := recordSpans(t)

	serveTraced(func(echo.Context) error { return echo.NewHTTPError(http.StatusNotFound) }, nil)

	if events := onlySpan(t, recorder).Events; len(events) != 0 {
		t.Fatalf("expected no events for a client error, got %+v", events)
	}
}
