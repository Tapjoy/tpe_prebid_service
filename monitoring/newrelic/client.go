package newrelic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	newrelic "github.com/newrelic/go-agent" // newrelic
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type client struct {
	httpClient httpClient
	tag        string
}

// NewClient ...
func NewClient(httpClient httpClient, opts ...ClientOption) (*client, error) {
	if httpClient == nil {
		return nil, errors.New("no http client provided")
	}

	// default
	c := client{
		httpClient: httpClient,
		tag:        "",
	}

	// optional
	for _, opt := range opts {
		err := opt(&c)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// Do ...
func (c client) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	txn, ok := ctx.Value(txnKey).(newrelic.Transaction)
	if !ok {
		// no new relic transaction, so do normal request
		return c.httpClient.Do(req)
	}
	seg := makeExternalSegment(txn, req, c.tag)
	defer seg.End()

	resp, err := c.httpClient.Do(req)
	seg.Response = resp

	return resp, err
}

// new relic will automatically group similar transaction names (eg, localhost:8082)
// we avoid using/appending the request's host when tags are defined
// see Metric Grouping Issue (MGI)
// https://docs.newrelic.com/docs/agents/manage-apm-agents/troubleshooting/metric-grouping-issues
func makeExternalSegment(txn newrelic.Transaction, req *http.Request, tag string) *newrelic.ExternalSegment {
	if tag == "" {
		// default external segment URL, http.Request.URL.Host
		return newrelic.StartExternalSegment(txn, req)
	}

	port := req.URL.Port()
	if port != "" {
		tag = fmt.Sprintf("%s:%s", tag, port)
	}

	// use tag for external segment URL
	// requires protocol ("http://"), see https://github.com/newrelic/go-agent/blob/f5bce3387232559bcbe6a5f8227c4bf508dac1ba/segments.go#L58
	segment := newrelic.StartExternalSegment(txn, req)
	segment.URL = "http://" + tag

	return segment
}

// ClientOption ...
type ClientOption func(*client) error

// SegmentTag ...
// FIXME: deprecate OR actually use in appfactory
func SegmentTag(tag string) ClientOption {
	sanitize := func(in string) string {
		out := strings.Replace(in, " ", "_", -1)
		return out
	}

	return func(c *client) error {
		if tag == "" {
			return errors.New("no tag provided")
		}

		tag = sanitize(tag)
		c.tag = tag
		return nil
	}
}
