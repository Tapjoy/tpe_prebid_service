package http

import (
	"fmt"
	"net/http"

	"github.com/tapjoy/go-tracing/tracing"
	"go.opentelemetry.io/otel/api/key"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

// WrapRoundTripper creates an http.RoundTripper to instrument external requests
// and add distributed tracing headers.  The http.RoundTripper returned creates
// an new span before delegating to the original http.RoundTripper
// provided (or http.DefaultTransport if none is provided)
func WrapRoundTripper(original http.RoundTripper, tracer apitrace.Tracer, config Config) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		cfg := config.Get(Endpoint(req.URL.Host + req.URL.Path))

		// The specification of http.RoundTripper requires that the request is never modified.
		req = cloneRequest(req)

		ctx, span := tracer.Start(
			req.Context(),
			"http/"+req.Host,
		)
		defer span.End()

		// inject propogation header into req
		ctx, req = httptrace.W3C(ctx, req)
		httptrace.Inject(ctx, req)

		// add configured request attributes
		addReqAttributes(span, req, cfg.request)

		if nil == original {
			original = http.DefaultTransport
		}
		resp, err := original.RoundTrip(req)

		if resp != nil {
			span.SetAttributes(key.Int(fmt.Sprintf("resp.status_code"), resp.StatusCode))
		}

		if err != nil {
			span.SetAttributes(key.String(fmt.Sprintf("resp.error"), err.Error()))
			return resp, err
		}

		// add configured response attributes
		addRespAttributes(span, resp, cfg.response)

		return resp, nil
	})
}

func addReqAttributes(span apitrace.Span, req *http.Request, reqCfg ReqCfg) {
	span.SetAttributes(
		key.String(fmt.Sprintf("req.host"), req.URL.Host),
		key.String(fmt.Sprintf("req.path"), req.URL.Path),
	)

	// Add parameters if configured for this request
	if reqCfg.params {
		params := req.URL.Query()
		for k, value := range params {
			// is there at least one value or empty for param
			if len(value) < 1 || value[0] == "" {
				continue
			}
			span.SetAttributes(key.String(fmt.Sprintf("req.params.%s", k), value[0]))
		}
	}

	reqMetaData := readReqBody(req, reqCfg.body)
	span.SetAttributes(key.Int(fmt.Sprintf("req.size"), reqMetaData.ContentSize))

	// add value to span
	tracing.AddInterfaceToSpan(span, fmt.Sprintf("req.body"), reqMetaData.Value)
}

func addRespAttributes(span apitrace.Span, resp *http.Response, respCfg RespCfg) {
	respMetaData := readRespBody(resp, respCfg.body)

	span.SetAttributes(
		key.Int(fmt.Sprintf("resp.size"), respMetaData.ContentSize),
	)

	// add value to span
	tracing.AddInterfaceToSpan(span, fmt.Sprintf("resp.body"), respMetaData.Value)
}

// cloneRequest mimics implementation of
// https://godoc.org/github.com/google/go-github/github#BasicAuthTransport.RoundTrip
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
