package http

// Endpoint is the host and path of an http request
type Endpoint string // host + path

// Config is all the configurations for this http sender
type Config struct {
	defaultCfg EndpointConfig

	endpointCfgs map[Endpoint]EndpointConfig
}

// Get returns the specifc HTTPConfig or defaultCfg
func (c Config) Get(e Endpoint) EndpointConfig {
	if cfg, ok := c.endpointCfgs[e]; ok {
		return cfg
	}
	return c.defaultCfg
}

// EndpointConfig defines tracing configs for an endpoint
type EndpointConfig struct {
	request  ReqCfg
	response RespCfg
}

// ReqCfg specifies what to log from the request
type ReqCfg struct {
	params bool // add parameters to span information

	body Unmarshaler // add request body to span information
}

// RespCfg specifies what to log from the response
type RespCfg struct {
	body Unmarshaler // add response body to span information
}

// Unmarshaler ...
type Unmarshaler interface {
	Unmarshal(data []byte) (interface{}, error)
}

// RawUnmarshaler is an implementation of []byte to string
type RawUnmarshaler struct{}

// Unmarshal turns []byte into a string
func (RawUnmarshaler) Unmarshal(data []byte) (interface{}, error) {
	return string(data), nil
}

// NewCfg returns a configuration for http.Client or http.RoundTripper
func NewCfg(d EndpointConfig, eMap map[Endpoint]EndpointConfig) Config {
	return Config{
		defaultCfg:   d,
		endpointCfgs: eMap,
	}
}

// NewEndpointCfg returns a configuration for an endpoint
func NewEndpointCfg(reqParams bool, req, resp Unmarshaler) EndpointConfig {
	return EndpointConfig{
		request: ReqCfg{
			params: reqParams,
			body:   req,
		},
		response: RespCfg{
			body: resp,
		},
	}
}
