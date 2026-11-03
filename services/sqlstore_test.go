package services

import (
	"context"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

func newTracedSQLStore(t *testing.T) *sqlstore.Container {
	t.Helper()
	address := "file:" + filepath.Join(t.TempDir(), "store.db") + "?_foreign_keys=on"
	container, err := openSQLStore(context.Background(), "sqlite3", address)
	if err != nil {
		t.Fatalf("open sqlstore: %v", err)
	}
	t.Cleanup(func() { _ = container.Close() })
	return container
}

func spanNamed(spans tracetest.SpanStubs, name string) (tracetest.SpanStub, bool) {
	for _, span := range spans {
		if span.Name == name {
			return span, true
		}
	}
	return tracetest.SpanStub{}, false
}

func TestSQLStoreUpgradesWithoutTracingTheBoot(t *testing.T) {
	recorder := recordSpans(t)

	newTracedSQLStore(t)

	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans without a parent, got %d", len(spans))
	}
}

func TestSQLStoreSkipsSpanWithoutParent(t *testing.T) {
	container := newTracedSQLStore(t)
	recorder := recordSpans(t)

	if _, err := container.GetAllDevices(context.Background()); err != nil {
		t.Fatalf("get devices: %v", err)
	}

	if spans := recorder.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans without a parent, got %d", len(spans))
	}
}

func TestSQLStoreCreatesQuerySpanWithRowsAsChild(t *testing.T) {
	container := newTracedSQLStore(t)
	recorder := recordSpans(t)
	ctx := startParentSpan(t)

	if _, err := container.GetAllDevices(ctx); err != nil {
		t.Fatalf("get devices: %v", err)
	}

	spans := recorder.GetSpans()
	query, ok := spanNamed(spans, "sql.conn.query")
	if !ok {
		t.Fatalf("missing query span, got %d spans", len(spans))
	}
	rows, ok := spanNamed(spans, "sql.rows")
	if !ok {
		t.Fatalf("missing rows span, got %d spans", len(spans))
	}
	if query.Parent.SpanID() != trace.SpanContextFromContext(ctx).SpanID() {
		t.Error("query span is not a child of the caller span")
	}
	if rows.Parent.SpanID() != query.SpanContext.SpanID() {
		t.Error("rows span is not a child of the query span")
	}
	if !hasAttribute(query.Attributes, semconv.DBSystemNameSQLite) {
		t.Error("missing db.system.name=sqlite")
	}
}
