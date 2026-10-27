package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	TraceResponseHeader = "traceresponse"
	traceParentHeader   = "traceparent"
)

var tracer = otel.Tracer("github.com/verbeux-ai/whatsmiau/server/middleware")

func Tracing(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.IsWebSocket() {
			return next(c)
		}

		req := c.Request()
		ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		ctx, span := tracer.Start(ctx, req.Method+" "+c.Path(),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				semconv.HTTPRoute(c.Path()),
			),
		)
		defer span.End()

		if instanceID := routeInstanceID(c); instanceID != "" {
			span.SetAttributes(whatsmiau.InstanceIDKey.String(instanceID))
			ctx = whatsmiau.ContextWithInstanceID(ctx, instanceID)
		}

		c.SetRequest(req.WithContext(ctx))
		setTraceResponse(ctx, c.Response().Header())

		defer func() {
			if recovered := recover(); recovered != nil {
				span.SetAttributes(semconv.HTTPResponseStatusCode(http.StatusInternalServerError))
				span.SetStatus(codes.Error, "panic")
				panic(recovered)
			}
		}()

		err := next(c)

		status := responseStatus(c, err)
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
			if err != nil {
				span.RecordError(err)
			}
		}

		return err
	}
}

func routeInstanceID(c echo.Context) string {
	if instanceID := c.Param("instance"); instanceID != "" {
		return instanceID
	}
	return c.Param("id")
}

func setTraceResponse(ctx context.Context, header http.Header) {
	carrier := propagation.HeaderCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if traceParent := carrier.Get(traceParentHeader); traceParent != "" {
		header.Set(TraceResponseHeader, traceParent)
	}
}

func responseStatus(c echo.Context, err error) int {
	if err == nil || c.Response().Committed {
		return c.Response().Status
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}
	return http.StatusInternalServerError
}
