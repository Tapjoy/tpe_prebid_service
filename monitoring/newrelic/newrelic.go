package newrelic

import (
	"errors"
	"fmt"

	newrelic "github.com/newrelic/go-agent"
	"github.com/prebid/prebid-server/config"
)

type Application interface {
	newrelic.Application
	Enabled() bool
}

type app struct {
	newrelic.Application
	enabled bool
}

func (a app) Enabled() bool {
	return a.enabled
}

// NewApplication ...
func NewApplication(cfg config.NewRelic) (Application, error) {

	enabled := len(cfg.AppName) > 0 && len(cfg.LicenseKey) > 0
	if !enabled {
		// use null object if newrelic undefined
		noop, err := newNOOP()
		if err != nil {
			return nil, fmt.Errorf("error instantiating noop newrelic application: %s", err)
		}

		return app{
			Application: noop,
			enabled:     false,
		}, nil
	}

	nrApp, err := makeApplication(cfg.AppName, cfg.LicenseKey, cfg.DebugMode)
	if err != nil {
		return nil, fmt.Errorf("error instantiating newrelic.Application: %s", err)
	}

	return app{
		Application: nrApp,
		enabled:     true,
	}, nil
}

func makeApplication(appName, licenseKey string, debug bool) (newrelic.Application, error) {
	if appName == "" {
		return nil, errors.New("no app name provided")
	}
	if licenseKey == "" {
		return nil, errors.New("no license key provided")
	}

	cfg := newrelic.NewConfig(appName, licenseKey)
	cfg.Enabled = true
	// distributed tracing and cross application tracing cannot be used simultaneously.
	cfg.CrossApplicationTracer.Enabled = false
	cfg.DistributedTracer.Enabled = true
	cfg.ErrorCollector.IgnoreStatusCodes = []int{422, 502} // 422(unprocessable entity), 502(bad gateway)

	/*
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		cfg.Logger = logrus.StandardLogger()
	*/
	return newrelic.NewApplication(cfg)
}

func newNOOP() (newrelic.Application, error) {
	return newrelic.NewApplication(
		newrelic.Config{
			Enabled: false,
		},
	)
}
