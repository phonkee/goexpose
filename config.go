package goexpose

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

/*
Returns filename from file
*/
func NewConfigFromFilename(filename string) (config *Config, err error) {
	config = NewConfig()

	var (
		result []byte
	)
	if result, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	// unmarshal config
	if err = json.Unmarshal(result, config); err != nil {
		return
	}

	// get config dir
	if config.Directory, err = filepath.Abs(filepath.Dir(filename)); err != nil {
		return
	}

	return
}

/*
Returns config with default values
*/
func NewConfig() *Config {
	return &Config{
		Host: "0.0.0.0",
		Port: 9980,
	}
}

/*
Main config
*/
type Config struct {
	Host        string                       `json:"host"`
	Port        int                          `json:"port"`
	SSL         *SSLConfig                   `json:"ssl"`
	PrettyJson  bool                         `json:"pretty_json"`
	Authorizers map[string]*AuthorizerConfig `json:"authorizers"`
	Endpoints   []*EndpointConfig            `json:"endpoints"`
	ReloadEnv   bool                         `json:"reload_env"`
	Directory   string                       `json:"-"`
}

/*
SSL config
*/
type SSLConfig struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

/*
Task config
*/
type TaskConfig struct {
	Type        string          `json:"type"`
	Authorizers []string        `json:"authorizers"`
	Config      json.RawMessage `json:"config"`
	QueryParams *QueryParams    `json:"query_params"`
	Description string          `json:"description"`
}

type EndpointConfig struct {
	Authorizers []string              `json:"authorizers"`
	Path        string                `json:"path"`
	Methods     map[string]TaskConfig `json:"methods"`
	Type        string                `json:"type"`
	QueryParams *QueryParams          `json:"query_params"`
	RawResponse bool                  `json:"raw_response"`
}

func (e *EndpointConfig) Validate() (err error) {

	if e.QueryParams != nil {
		if err = e.QueryParams.Validate(); err != nil {
			return
		}
	}

	// set type to unset tasks
	e.Type = strings.TrimSpace(e.Type)
	if e.Type != "" {
		for _, tc := range e.Methods {
			if tc.Type == "" {
				tc.Type = e.Type
			}
		}
	}

	return
}

func (e *EndpointConfig) RouteName() string {
	hash := sha256.New()
	io.WriteString(hash, e.Path)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

/*
Validate method validates task config
*/
func (t *TaskConfig) Validate() (err error) {
	t.Type = strings.TrimSpace(t.Type)
	t.Description = strings.TrimSpace(t.Description)
	if t.Type == "" {
		return fmt.Errorf("Invalid task type")
	}

	if t.QueryParams != nil {
		if err = t.QueryParams.Validate(); err != nil {
			return
		}
	}

	return
}

/*
Configuration for authorizer
*/
type AuthorizerConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

/*
Validate
*/
func (a *AuthorizerConfig) Validate() (err error) {
	if ok := AuthorizerExists(a.Type); !ok {
		err = fmt.Errorf("authorizer %s does not exist", a.Type)
	}

	return
}

/*
Query params
*/

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

/*
Returns params from request
*/
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

/*
Param config
*/
type QueryParamsConfigParam struct {
	Name    string `json:"name"`
	Regexp  string `json:"regexp"`
	Default string `json:"default"`

	// compiled regexp
	compiled *regexp.Regexp
}
