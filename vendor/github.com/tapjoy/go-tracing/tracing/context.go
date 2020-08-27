package tracing

import (
	"context"

	"go.opentelemetry.io/otel/api/core"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"golang.org/x/sync/syncmap"
)

type ctxKey string

var TraceKey = ctxKey("tj-traceID")

var traceMap = syncmap.Map{}

// AddInterfaceToSpan adds a complex data type to the passed in span
// For structs, it adds each exported field
// For maps, it adds each key/value
func AddInterfaceToSpan(span apitrace.Span, key string, data interface{}) {
	if span == nil {
		return
	}

	span.SetAttributes(getKeyValues(key, data)...)
}

// AddInterfaceToActiveSpan adds a complex data type to the active span
// For structs, it adds each exported field
// For maps, it adds each key/value
func AddInterfaceToActiveSpan(ctx context.Context, key string, data interface{}) {
	span := apitrace.CurrentSpan(ctx)
	if span == nil {
		return
	}
	span.SetAttributes(getKeyValues(key, data)...)
}

// AddInterfaceToTrace adds a complex data type to the active Trace
// For structs, it adds each exported field
// For maps, it adds each key/value
func AddInterfaceToTrace(ctx context.Context, key string, data interface{}) {
	if ctx == nil {
		return
	}

	traceID, ok := ctx.Value(TraceKey).(string)
	if !ok {
		return
	}

	result := getKeyValues(key, data)

	val, ok := traceMap.Load(traceID)
	if ok {
		kvs, ok := val.([]core.KeyValue)
		if ok {
			result = append(result, kvs...)
		}
	}

	traceMap.Store(traceID, result)
}

// AddKeyValueToTrace adds a core.KeyValue type to the active Trace
func AddKeyValueToTrace(ctx context.Context, kv core.KeyValue) {
	if ctx == nil {
		return
	}

	traceID, ok := ctx.Value(TraceKey).(string)
	if !ok {
		return
	}

	result := []core.KeyValue{kv}
	val, ok := traceMap.Load(traceID)
	if ok {
		kvs, ok := val.([]core.KeyValue)
		if ok {
			result = append(kvs, kv)
		}
	}

	traceMap.Store(traceID, result)
}

// GetTraceData returns core.KeyValue's associated with trace ID
func GetTraceData(traceID string) []core.KeyValue {
	val, ok := traceMap.Load(traceID)
	if !ok {
		return nil
	}

	kvs, ok := val.([]core.KeyValue)
	if !ok {
		return nil
	}

	return kvs
}

// RemoveTraceData removes core.KeyValue's associated with trace ID
func RemoveTraceData(traceID string) {
	traceMap.Delete(traceID)
}
