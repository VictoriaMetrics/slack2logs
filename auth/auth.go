package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

// HTTPClientConfig represents http client config.
type HTTPClientConfig struct {
	BasicAuth *BasicAuthConfig
}

// NewConfig creates auth config for the given hcc.
func (hcc *HTTPClientConfig) NewConfig() (*Config, error) {
	opts := &Options{
		BasicAuth: hcc.BasicAuth,
	}

	return opts.NewConfig()
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string
	Password     string
	PasswordFile string
}

// ConfigOptions options which helps build Config
type ConfigOptions func(config *HTTPClientConfig)

// Generate returns Config based on the given params
func Generate(filterOptions ...ConfigOptions) (*Config, error) {
	authCfg := &HTTPClientConfig{}
	for _, option := range filterOptions {
		option(authCfg)
	}

	return authCfg.NewConfig()
}

// WithBasicAuth returns AuthConfigOptions and initialized BasicAuthConfig based on given params
func WithBasicAuth(username, password string) ConfigOptions {
	return func(config *HTTPClientConfig) {
		if username != "" || password != "" {
			config.BasicAuth = &BasicAuthConfig{
				Username: username,
				Password: password,
			}
		}
	}
}

// Config is auth config.
type Config struct {
	getAuthHeader func() string
	headers       []keyValue
}

// SetHeaders sets the configured ac headers to req.
func (ac *Config) SetHeaders(req *http.Request, setAuthHeader bool) {
	reqHeaders := req.Header
	for _, h := range ac.headers {
		reqHeaders.Set(h.key, h.value)
	}
	if setAuthHeader {
		if ah := ac.GetAuthHeader(); ah != "" {
			reqHeaders.Set("Authorization", ah)
		}
	}
}

// GetAuthHeader returns optional `Authorization: ...` http header.
func (ac *Config) GetAuthHeader() string {
	f := ac.getAuthHeader
	if f == nil {
		return ""
	}
	return f()
}

type authContext struct {
	// getAuthHeader must return <value> for 'Authorization: <value>' http request header
	getAuthHeader func() string

	// authDigest must contain the digest for the used authorization
	// The digest must be changed whenever the original config changes.
	authDigest string
}

func (ac *authContext) initFromBasicAuthConfig(ba *BasicAuthConfig) error {
	if ba.Username == "" {
		return fmt.Errorf("missing `username`")
	}
	if ba.Password != "" {
		ac.getAuthHeader = func() string {
			token := ba.Username + ":" + ba.Password
			token64 := base64.StdEncoding.EncodeToString([]byte(token))
			return "Basic " + token64
		}
		ac.authDigest = fmt.Sprintf("basic(username=%q, password=%q)", ba.Username, ba.Password)
		return nil
	}
	return nil
}

// Options contain options, which must be passed to NewConfig.
type Options struct {
	// BasicAuth contains optional BasicAuthConfig.
	BasicAuth *BasicAuthConfig
}

// NewConfig creates auth config from the given opts.
func (opts *Options) NewConfig() (*Config, error) {
	var ac authContext
	if opts.BasicAuth != nil {
		if ac.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot use both `authorization`")
		}
		if opts.BasicAuth.Username == "" {
			return nil, fmt.Errorf("missing `username` for basic authorization")
		}

		if err := ac.initFromBasicAuthConfig(opts.BasicAuth); err != nil {
			return nil, err
		}
	}

	c := &Config{
		getAuthHeader: ac.getAuthHeader,
	}
	return c, nil
}

type keyValue struct {
	key   string
	value string
}
