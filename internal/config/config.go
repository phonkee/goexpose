package config

import (
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-defaults"
	"github.com/phonkee/goexpose/domain"
	"net/http"
	"regexp"
	"strings"
)

// NewConfig returns config with default values
func NewConfig() *Config {
	result := Config{}

	defaults.SetDefaults(&result)

	return &result
}

// Config is main configuration for goexpose
type Config struct {
	Host        string                       `json:"host" default:"0.0.0.0"`
	Port        int                          `json:"port" default:"9980"`
	SSL         *SSLConfig                   `json:"ssl"`
	PrettyJson  bool                         `json:"pretty_json"`
	Authorizers map[string]*AuthorizerConfig `json:"authorizers"`
	Endpoints   []*domain.EndpointConfig     `json:"endpoints"`
	ReloadEnv   bool                         `json:"reload_env"`
	Directory   string                       `json:"-"`
	Raw         json.RawMessage              `json:"-"`
}

type SSLConfig struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// AuthorizerConfig configures authorizer
type AuthorizerConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

func (a *AuthorizerConfig) Validate() (err error) {
	// TODO: move into auth package
	//if ok := goexpose.AuthorizerExists(a.Type); !ok {
	//	err = fmt.Errorf("authorizer %s does not exist", a.Type)
	//}

	return
}

type QueryParams struct {
	ReturnParams bool                      `json:"return_params"`
	Params       []*QueryParamsConfigParam `json:"params"`
}

func (q *QueryParams) Validate() (err error) {

	var re *regexp.Regexp

	// validate query params
	for _, param := range q.Params {
		param.Name = strings.TrimSpace(param.Name)
		if param.Name == "" {
			return fmt.Errorf("query param name missing")
		}

		param.Regexp = strings.TrimSpace(param.Regexp)

		// regexp available, precompile it
		if param.Regexp != "" {
			if re, err = regexp.Compile(param.Regexp); err != nil {
				return fmt.Errorf("query param regexp %v returned %v", param.Regexp, err)
			}
			param.compiled = re
		}
	}

	return
}

// GetParams Returns params from request
func (q *QueryParams) GetParams(r *http.Request) (result map[string]string) {
	result = map[string]string{}

	if q == nil {
		return
	}

	for _, param := range q.Params {
		value := r.URL.Query().Get(param.Name)
		value = strings.TrimSpace(value)

		if value == "" {
			if param.Default != "" {
				result[param.Name] = param.Default
			}
		} else {
			if param.compiled != nil {

				if param.compiled.MatchString(value) {
					result[param.Name] = value
				} else {
					if param.Default != "" {
						result[param.Name] = param.Default
					}
				}
			} else {
				result[param.Name] = value
			}
		}
	}
	return
}

// QueryParamsConfigParam config
type QueryParamsConfigParam struct {
	Name    string `json:"name"`
	Regexp  string `json:"regexp"`
	Default string `json:"default"`

	// compiled regexp
	compiled *regexp.Regexp
}
