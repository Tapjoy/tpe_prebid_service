package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/tapjoy/go-tracing/tracing"

	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/key"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

type tracerHandler struct {
	name   string
	tracer apitrace.Tracer
}

// NewHandler ...
func NewHandler(name string, tracer apitrace.Tracer) (*tracerHandler, error) {
	if name == "" {
		return nil, errors.New("no name provided")
	}

	if tracer == nil {
		return nil, errors.New("no trace provider provided")
	}

	return &tracerHandler{
		name:   name,
		tracer: tracer,
	}, nil
}

// Middleware ...
func (s tracerHandler) Middleware(h http.Handler) http.Handler {
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		_, entries, spanCtx := httptrace.Extract(r.Context(), r)

		r = r.WithContext(distributedcontext.WithMap(r.Context(), distributedcontext.NewMap(distributedcontext.MapUpdate{
			MultiKV: entries,
		})))

		ctx, span := s.tracer.Start(
			r.Context(),
			s.name,
			apitrace.ChildOf(spanCtx),
		)
		defer span.End()

		traceJSON, _ := span.SpanContext().TraceID.MarshalJSON()
		traceID := string(traceJSON)
		defer tracing.RemoveTraceData(traceID) // cleanup trace data from memory

		// add trace id to context for children ServeHTTP to reference
		ctx = context.WithValue(ctx, tracing.TraceKey, traceID)
		r = r.WithContext(ctx)

		// add request attributes
		span.SetAttributes(key.String("name", s.name))
		addRequestPropsAttributes(span, r)

		// replace the writer with our wrapper to catch the status code
		wrappedWriter := tracing.NewResponseWriter(w)

		h.ServeHTTP(wrappedWriter.Wrapped, r)

		if wrappedWriter.Status == 0 {
			wrappedWriter.Status = 200
		}

		span.SetAttributes(key.Int("resp.status_code", wrappedWriter.Status))

		// add trace data to this span
		span.SetAttributes(tracing.GetTraceData(traceID)...)
	}
	return http.HandlerFunc(wrappedHandler)
}

func addRequestPropsAttributes(span apitrace.Span, req *http.Request) {
	span.SetAttributes(key.String("req.path", req.URL.Path))
	span.SetAttributes(key.String("req.method", req.Method))
	span.SetAttributes(key.String("req.host", req.Host))

	if hostname, err := os.Hostname(); err == nil {
		span.SetAttributes(key.String("req.hostname", hostname))
	}

	params := req.URL.Query()
	for k, value := range params {
		// is there at least one value or empty for param
		if len(value) < 1 || value[0] == "" {
			continue
		}
		span.SetAttributes(key.String(fmt.Sprintf("req.params.%s", k), value[0]))
	}
}
