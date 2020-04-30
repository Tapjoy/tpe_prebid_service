package context

import (
	"errors"
	"net/http"

	"golang.org/x/net/context/ctxhttp"
)

type clientWithContext struct {
	client *http.Client
}

// NewClient ...
func NewClient(client *http.Client, opts ...ClientOption) (*clientWithContext, error) {
	if client == nil {
		return nil, errors.New("no http.Client provided")
	}

	// default
	c := clientWithContext{
		client: client,
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
func (c clientWithContext) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	resp, err := ctxhttp.Do(ctx, c.client, req)

	return resp, err
}
