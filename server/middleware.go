package server

import (
	"net/http"

	"github.com/prebid/prebid-server/monitoring/newrelic"
)

// Middleware ...
type Middleware func(handler http.Handler) http.Handler

// MakeMiddleware ...
func MakeMiddleware(nrApp newrelic.Application) ([]Middleware, error) {
	middleware := []Middleware{}

	// middleware: new relic
	if nrApp.Enabled() {
		// order matters, `newrelic.Middleware` wraps `newrelic.WithTxn`
		// ensuring that the incoming request is a `newrelic.Transaction`
		middleware = append(middleware, newrelic.AddTxnParams)
		middleware = append(middleware, newrelic.WithTxn)
		middleware = append(middleware, newrelic.Middleware(nrApp))
	}

	return middleware, nil
}
