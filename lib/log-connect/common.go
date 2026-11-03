package log_connect

import (
	"context"

	"github.com/verbeux-ai/whatsmiau/env"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func StartLogger() error {
	if env.Env.GCLEnabled {
		c, err := startGCL()
		if err != nil {
			return err
		}
		zap.ReplaceGlobals(zap.New(*c))
		return nil
	}

	logger, err := zap.NewProduction()
	if env.Env.DebugMode {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)

	return nil
}

func WithTrace(ctx context.Context) *zap.Logger {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return zap.L()
	}

	return zap.L().With(
		zap.String("trace_id", spanContext.TraceID().String()),
		zap.String("span_id", spanContext.SpanID().String()),
	)
}
