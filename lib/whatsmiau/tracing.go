package whatsmiau

import (
	"context"
	"io"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const InstanceIDKey = attribute.Key("whatsmiau.instance.id")

var tracer = otel.Tracer("github.com/verbeux-ai/whatsmiau/lib/whatsmiau")

type instanceIDContextKey struct{}

func ContextWithInstanceID(ctx context.Context, instanceID string) context.Context {
	return context.WithValue(ctx, instanceIDContextKey{}, instanceID)
}

func startChildSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	parent := trace.SpanFromContext(ctx)
	if !parent.IsRecording() {
		return ctx, parent
	}
	if instanceID, ok := ctx.Value(instanceIDContextKey{}).(string); ok {
		opts = append(opts, trace.WithAttributes(InstanceIDKey.String(instanceID)))
	}
	return tracer.Start(ctx, name, opts...)
}

func sendMessage(ctx context.Context, client *whatsmeow.Client, to types.JID, message *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	ctx, span := startChildSpan(ctx, "whatsapp.send_message", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	res, err := client.SendMessage(skipStoreSpansOnFanOut(ctx, to), to, message, extra...)
	if span.IsRecording() {
		span.SetAttributes(sendTimingAttributes(res.DebugTimings)...)
	}
	if err != nil {
		span.SetStatus(codes.Error, "send message failed")
		return res, err
	}

	span.SetAttributes(semconv.MessagingMessageID(res.ID))
	return res, nil
}

func skipStoreSpansOnFanOut(ctx context.Context, to types.JID) context.Context {
	if to.Server != types.GroupServer && to.Server != types.BroadcastServer {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, trace.SpanContextFromContext(ctx))
}

func sendTimingAttributes(timings whatsmeow.MessageDebugTimings) []attribute.KeyValue {
	stages := []struct {
		name     string
		duration time.Duration
	}{
		{"lid_fetch", timings.LIDFetch},
		{"queue", timings.Queue},
		{"marshal", timings.Marshal},
		{"get_participants", timings.GetParticipants},
		{"get_devices", timings.GetDevices},
		{"group_encrypt", timings.GroupEncrypt},
		{"peer_encrypt", timings.PeerEncrypt},
		{"send", timings.Send},
		{"resp", timings.Resp},
		{"retry", timings.Retry},
	}

	attributes := make([]attribute.KeyValue, 0, len(stages))
	for _, stage := range stages {
		if stage.duration > 0 {
			attributes = append(attributes, attribute.Float64("whatsapp.send.timing."+stage.name+"_ms", float64(stage.duration)/float64(time.Millisecond)))
		}
	}
	return attributes
}

func uploadMedia(ctx context.Context, client *whatsmeow.Client, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	ctx, span := startChildSpan(ctx, "whatsapp.upload_media", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	res, err := client.Upload(ctx, data, mediaType)
	if err != nil {
		span.SetStatus(codes.Error, "upload media failed")
	}
	return res, err
}

type tracingTransport struct {
	operation string
	base      http.RoundTripper
}

func (t tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := startChildSpan(req.Context(), t.operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(req.Method),
			semconv.ServerAddress(req.URL.Hostname()),
		),
	)

	if !span.IsRecording() {
		return t.base.RoundTrip(req)
	}

	res, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.SetStatus(codes.Error, "request failed")
		span.End()
		return nil, err
	}

	span.SetAttributes(semconv.HTTPResponseStatusCode(res.StatusCode))
	if res.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, http.StatusText(res.StatusCode))
	}

	res.Body = &spanEndingBody{ReadCloser: res.Body, span: span}
	return res, nil
}

type spanEndingBody struct {
	io.ReadCloser
	span trace.Span
}

func (b *spanEndingBody) Close() error {
	defer b.span.End()
	return b.ReadCloser.Close()
}
