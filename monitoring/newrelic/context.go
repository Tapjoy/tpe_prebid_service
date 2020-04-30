package newrelic

import (
	"context"

	newrelic "github.com/newrelic/go-agent"
)

// GetTransaction ...
func GetTransaction(ctx context.Context) (newrelic.Transaction, bool) {
	txn, ok := ctx.Value(txnKey).(newrelic.Transaction)
	return txn, ok
}

// GoroutineTxn ...
func GoroutineTxn(ctx context.Context) (newrelic.Transaction, bool) {
	txn, ok := GetTransaction(ctx)
	if !ok {
		return nil, false
	}

	// TODO: is txn.NewGoroutine() guaranteed to return a transaction?
	return txn.NewGoroutine(), true
}

// GoroutineCtx returns a newrelic Transaction that supports goroutines
// if no Transaction is found or there is an error, the original context is returned
func GoroutineCtx(ctx context.Context) context.Context {
	txn, ok := GoroutineTxn(ctx)
	if !ok || txn == nil {
		// TODO: determine whether returning original context is appropriate on !ok
		return ctx
	}
	return context.WithValue(ctx, txnKey, txn)
}
