package tracing

import (
	"context"
	"fmt"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/sdk/export/trace"
)

type WrapExporter struct {
	hcExporter *honeycomb.Exporter
	traceMap   map[string][]core.KeyValue
}

func NewWrapExporter(hcExporter *honeycomb.Exporter) *WrapExporter {
	return &WrapExporter{
		hcExporter: hcExporter,
		traceMap:   make(map[string][]core.KeyValue),
	}
}

// ExportSpan exports a SpanData to Tracing.
func (e *WrapExporter) ExportSpan(ctx context.Context, data *trace.SpanData) {
	traceJSON, _ := data.SpanContext.TraceID.MarshalJSON()
	traceID := string(traceJSON)

	// apply attributes from trace
	data.Attributes = append(data.Attributes, GetTraceData(traceID)...)

	att := key.Int("meta.event.size", len(fmt.Sprintf("%+v", data.Attributes)))
	data.Attributes = append(data.Attributes, att)

	e.hcExporter.ExportSpan(ctx, data)
}

func (e *WrapExporter) Close() {
	e.hcExporter.Close()
}
