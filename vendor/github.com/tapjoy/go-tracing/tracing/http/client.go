package http

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tapjoy/go-tracing/tracing"
	"go.opentelemetry.io/otel/api/key"

	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type client struct {
	tag string
	cfg Config

	tracer     apitrace.Tracer
	httpClient httpClient
}

// NewHTTPClient ...
func NewHTTPClient(httpClient httpClient, tracer apitrace.Tracer, tag string, cfg Config) (*client, error) {
	if httpClient == nil {
		return nil, errors.New("no httpClient passed to tracing NewHTTPClient")
	}

	return &client{
		tracer:     tracer,
		httpClient: httpClient,
		tag:        tag + "-http",
		cfg:        cfg,
	}, nil
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	cfg := c.cfg.Get(Endpoint(req.URL.Host + req.URL.Path))

	ctx, span := c.tracer.Start(
		req.Context(),
		c.tag,
	)
	defer span.End()

	// inject propogation header into req
	ctx, req = httptrace.W3C(ctx, req)
	httptrace.Inject(ctx, req)

	// add configured request attributes
	c.addReqAttributes(span, req, cfg.request)

	// perform request
	resp, err := c.httpClient.Do(req)

	if resp != nil {
		span.SetAttributes(key.Int(fmt.Sprintf("resp.status_code"), resp.StatusCode))
	}

	if err != nil {
		span.SetAttributes(key.String(fmt.Sprintf("resp.error"), err.Error()))
		return resp, err
	}

	// add configured response attributes
	c.addRespAttributes(span, resp, cfg.response)

	return resp, nil
}

type reqMetaData struct {
	ContentSize int
	Value       interface{}
}

func (c *client) addReqAttributes(span apitrace.Span, req *http.Request, reqCfg ReqCfg) {
	// Always add host and path
	span.SetAttributes(
		key.String(fmt.Sprintf("req.host"), req.URL.Host),
		key.String(fmt.Sprintf("req.path"), req.URL.Path),
	)

	// Add parameters is configutred for this request
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

	// If instance of unmarshaler interface defined
	if bodyUnmarshaler != nil {
		requestValue, err := bodyUnmarshaler.Unmarshal(bodyBytes)
		if err != nil {
			// fallback to just string
			metaData.Value = string(bodyBytes)
		} else {
			metaData.Value = requestValue
		}
	}
	return metaData
}

func (c *client) addRespAttributes(span apitrace.Span, resp *http.Response, respCfg RespCfg) {
	respMetaData := readRespBody(resp, respCfg.body)

	span.SetAttributes(
		key.Int(fmt.Sprintf("resp.size"), respMetaData.ContentSize),
	)

	// add value to span
	tracing.AddInterfaceToSpan(span, fmt.Sprintf("resp.body"), respMetaData.Value)
}

func readRespBody(res *http.Response, bodyUnmarshaler Unmarshaler) reqMetaData {
	metaData := reqMetaData{
		ContentSize: 0,
		Value:       "",
	}

	if res.Body == nil {
		return metaData
	}

	// read and close body
	bodyBytes, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	// record inital size
	metaData.ContentSize = len(bodyBytes)

	// reset body to be able to be read by other middleware
	res.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

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
