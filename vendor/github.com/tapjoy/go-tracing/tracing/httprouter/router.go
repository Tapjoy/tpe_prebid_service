package httprouter

// Use this package to instrument inbound requests handled by a
// httprouter.Router. Use an *nrhttprouter.Router in place of your
// *httprouter.Router.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/tapjoy/go-tracing/tracing"
	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/key"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

type router interface {
	DELETE(path string, h httprouter.Handle)
	GET(path string, h httprouter.Handle)
	HEAD(path string, h httprouter.Handle)
	OPTIONS(path string, h httprouter.Handle)
	PATCH(path string, h httprouter.Handle)
	POST(path string, h httprouter.Handle)
	PUT(path string, h httprouter.Handle)

	Handle(method, path string, h httprouter.Handle)
	Handler(method, path string, handler http.Handler)
	HandlerFunc(method, path string, handler http.HandlerFunc)
	ServeHTTP(w http.ResponseWriter, req *http.Request)

	ServeFiles(path string, root http.FileSystem)
}

// Router should be used in place of httprouter.Router.  Create it using
// New().
type Router struct {
	Router router

	name          string
	pathBlacklist map[string]bool
	tracer        apitrace.Tracer
}

// WrapRouter ware a new Router interface with tracing to be used in place of httprouter.Router.
func WrapRouter(name string, tracer apitrace.Tracer, router router, pathBlacklist map[string]bool) (router, error) {
	if name == "" {
		return nil, errors.New("no name provided")
	}

	if tracer == nil {
		return nil, errors.New("no trace provider provided")
	}

	if router == nil {
		return nil, errors.New("no router provider provided")
	}

	if pathBlacklist == nil {
		pathBlacklist = map[string]bool{}
	}

	return &Router{
		Router: router,

		name:          name,
		tracer:        tracer,
		pathBlacklist: pathBlacklist,
	}, nil
}

func (r *Router) handle(method string, path string, original httprouter.Handle) {
	wrappedReqHandler := original

	if !r.pathBlacklist[path] {
		wrappedReqHandler = func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			_, entries, spanCtx := httptrace.Extract(req.Context(), req)

			req = req.WithContext(distributedcontext.WithMap(req.Context(), distributedcontext.NewMap(distributedcontext.MapUpdate{
				MultiKV: entries,
			})))

			ctx, span := r.tracer.Start(
				req.Context(),
				r.name,
				apitrace.ChildOf(spanCtx),
			)
			defer span.End()

			traceJSON, _ := span.SpanContext().TraceID.MarshalJSON()
			traceID := string(traceJSON)
			defer tracing.RemoveTraceData(traceID) // cleanup trace data from memory

			// add trace id to context for children ServeHTTP to reference
			ctx = context.WithValue(ctx, tracing.TraceKey, traceID)
			req = req.WithContext(ctx)

			// add request attributes
			span.SetAttributes(key.String("name", r.name))
			addRequestPropsAttributesHTTPRouter(span, req, ps)

			// replace the writer with our wrapper to catch the status code
			wrappedWriter := tracing.NewResponseWriter(w)

			original(wrappedWriter.Wrapped, req, ps)

			if wrappedWriter.Status == 0 {
				wrappedWriter.Status = 200
			}

			span.SetAttributes(key.Int("resp.status_code", wrappedWriter.Status))

			// add trace data to this span
			span.SetAttributes(tracing.GetTraceData(traceID)...)
		}
	}
	r.Router.Handle(method, path, wrappedReqHandler)
}

func addRequestPropsAttributesHTTPRouter(span apitrace.Span, req *http.Request, ps httprouter.Params) {
	span.SetAttributes(key.String("req.path", req.URL.Path))
	span.SetAttributes(key.String("req.method", req.Method))
	span.SetAttributes(key.String("req.host", req.Host))

	if hostname, err := os.Hostname(); err == nil {
		span.SetAttributes(key.String("req.hostname", hostname))
	}

	for _, param := range ps {
		k := param.Key
		v := param.Value
		// is there at least one value or empty for param
		if v == "" {
			continue
		}
		span.SetAttributes(key.String(fmt.Sprintf("req.params.%s", k), v))
	}

	reqMetaData := readReqBody(req, &RawUnmarshaler{})
	span.SetAttributes(key.Int(fmt.Sprintf("req.size"), reqMetaData.ContentSize))

	//tracing.AddInterfaceToSpan(span, fmt.Sprintf("req.body"), reqMetaData.Value)
}

type Unmarshaler interface {
	Unmarshal(data []byte) (interface{}, error)
}

type RawUnmarshaler struct{}

func (RawUnmarshaler) Unmarshal(data []byte) (interface{}, error) {
	return string(data), nil
}

type reqMetaData struct {
	ContentSize int
	Value       interface{}
}

func readReqBody(req *http.Request, bodyUnmarshaler Unmarshaler) reqMetaData {
	metaData := reqMetaData{
		ContentSize: 0,
		Value:       "",
	}

	if req.Body == nil {
		return metaData
	}

	// read and close body
	bodyBytes, _ := ioutil.ReadAll(req.Body)
	req.Body.Close()

	// record inital size
	metaData.ContentSize = len(bodyBytes)

	// reset body to be able to be read by other middleware
	req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	if bodyUnmarshaler != nil {
		requestValue, err := bodyUnmarshaler.Unmarshal(bodyBytes)
		if err != nil {
			metaData.Value = string(bodyBytes)
		} else {
			metaData.Value = requestValue
		}
	}
	return metaData
}

// DELETE replaces httprouter.Router.DELETE.
func (r *Router) DELETE(path string, h httprouter.Handle) {
	r.handle(http.MethodDelete, path, h)
}

// GET replaces httprouter.Router.GET.
func (r *Router) GET(path string, h httprouter.Handle) {
	r.handle(http.MethodGet, path, h)
}

// HEAD replaces httprouter.Router.HEAD.
func (r *Router) HEAD(path string, h httprouter.Handle) {
	r.handle(http.MethodHead, path, h)
}

// OPTIONS replaces httprouter.Router.OPTIONS.
func (r *Router) OPTIONS(path string, h httprouter.Handle) {
	r.handle(http.MethodOptions, path, h)
}

// PATCH replaces httprouter.Router.PATCH.
func (r *Router) PATCH(path string, h httprouter.Handle) {
	r.handle(http.MethodPatch, path, h)
}

// POST replaces httprouter.Router.POST.
func (r *Router) POST(path string, h httprouter.Handle) {
	r.handle(http.MethodPost, path, h)
}

// PUT replaces httprouter.Router.PUT.
func (r *Router) PUT(path string, h httprouter.Handle) {
	r.handle(http.MethodPut, path, h)
}

// Handle replaces httprouter.Router.Handle.
func (r *Router) Handle(method, path string, h httprouter.Handle) {
	r.handle(method, path, h)
}

// Handler replaces httprouter.Router.Handler.
func (r *Router) Handler(method, path string, handler http.Handler) {
	r.handle(method, path,
		func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
			if len(p) > 0 {
				ctx := req.Context()
				ctx = context.WithValue(ctx, httprouter.ParamsKey, p)
				req = req.WithContext(ctx)
			}
			handler.ServeHTTP(w, req)
		},
	)
}

// HandlerFunc replaces httprouter.Router.HandlerFunc.
func (r *Router) HandlerFunc(method, path string, handler http.HandlerFunc) {
	r.Handler(method, path, handler)
}

// ServeFiles replaces httprouter.Router.ServeFiles.
func (r *Router) ServeFiles(path string, root http.FileSystem) {
	r.Router.ServeFiles(path, root)
}

// ServeHTTP replaces httprouter.Router.ServeHTTP.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Router.ServeHTTP(w, req)
}
