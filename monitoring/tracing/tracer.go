package tracing

import (
	"fmt"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"github.com/prebid/prebid-server/config"
	"github.com/tapjoy/go-tracing/tracing"
	apitrace "go.opentelemetry.io/otel/api/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTraceProvider creted a new opentel trace provided with honeyconb exporter
func NewTraceProvider(cfg config.HoneyComb) (apitrace.Provider, func(), error) {
	// initalize honeycomb exporter
	hcExporter, err := honeycomb.NewExporter(honeycomb.Config{
		ApiKey:      cfg.WriteKey,
		Dataset:     cfg.WaterfallDatasetName,
		Debug:       cfg.Debug,
		ServiceName: cfg.ServiceName,
	})
	if err != nil {
		return apitrace.NoopProvider{}, nil, fmt.Errorf("error making hoenycomb exporter for opentelemetry: %s", err)
	}

	// wrap honeycomb exporter with internal exporter
	exporter := tracing.NewWrapExporter(hcExporter)

	// create trace provider
	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(
		sdktrace.Config{
			DefaultSampler:       sdktrace.ProbabilitySampler(cfg.SampleRate),
			MaxAttributesPerSpan: 100,
		}),
		sdktrace.WithSyncer(exporter))
	if err != nil {
		return apitrace.NoopProvider{}, nil, fmt.Errorf("error making provider for opentelemetry : %s", err)
	}

	return tp, exporter.Close, nil
}
