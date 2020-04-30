package newrelic

type ctxKey string

// default key mapped to the `newrelic.Transaction` in the request-scoped `context.Context`
const txnKey = ctxKey("newrelicTxn")
