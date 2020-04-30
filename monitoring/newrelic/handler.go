package newrelic

import (
	"context"
	"net/http"

	newrelic "github.com/newrelic/go-agent"
)

// Middleware starts a newrelic transaction for a given http.Handler
func Middleware(app newrelic.Application) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := r.URL.Path
			txn := app.StartTransaction(pattern, w, r)
			defer txn.End()

			// a newrelic transaction masquerades as an http.ResponseWriter
			handler.ServeHTTP(txn, r)
		})
	}
}

// WithTxn adds the newrelic transaction (defined by Middleware) to the context
func WithTxn(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// relies on `Middleware` to ensure the `http.ResponseWriter` is a `newrelic.Transaction`
		ctx := context.WithValue(r.Context(), txnKey, w)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AddTxnParams ...
func AddTxnParams(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		txn, ok := ctx.Value(txnKey).(newrelic.Transaction)
		if !ok {
			// not newrelic transaction, server normally
			handler.ServeHTTP(w, r)
			return
		}

		// newrelic transaction, add request attributes to transaction
		for k, vSlice := range r.URL.Query() {
			value := ""
			for _, v := range vSlice {
				switch len(value) {
				case 0:
					value = v
				default:
					value = value + ", " + v
				}
			}

			err := txn.AddAttribute(k, value)
			if err != nil {
				// TODO: handle error
			}
		}
		handler.ServeHTTP(w, r)
	})
}
