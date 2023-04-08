package goexpose

import (
	"encoding/json"
	"fmt"
	"github.com/phonkee/goexpose/domain"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"errors"
	"github.com/ghodss/yaml"
)

type unmarshalFunc func([]byte, interface{}) error

var (
	configFormats     map[string]unmarshalFunc
	configFormatsLock *sync.RWMutex
)

func init() {

	// prepare configuration formats (currently json, yaml)
	configFormats = map[string]unmarshalFunc{}
	configFormatsLock = &sync.RWMutex{}

	func() {
		configFormatsLock.Lock()
		defer configFormatsLock.Unlock()
		configFormats["json"] = json.Unmarshal

		// custom yaml unmarshal, since when used directly it panics.
		// so we just convert yaml to json and call json unmarshal
		configFormats["yaml"] = func(body []byte, target interface{}) (err error) {
			if response, e := yaml.YAMLToJSON(body); e != nil {
				err = e
			} else {
				err = json.Unmarshal(response, target)
			}

			return
		}
	}()
}

/*
Returns filename from file
*/
func NewConfigFromFilename(filename, format string) (config *Config, err error) {
	config = NewConfig()

	var (
		result []byte
	)
	if result, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	found := false
	for name, fmtUnmarshalFunc := range configFormats {
		if name == format {
			if err = fmtUnmarshalFunc(result, config); err != nil {
				return
			}
			found = true
			break
		}
	}

	if !found {
		err = errors.New("file format not found")
		return
	}

	// unmarshal config
	//if err = json.Unmarshal(result, config); err != nil {
	//	return
	//}

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

// Config is main configuration for goexpose
type Config struct {
	Host        string                       `json:"host"`
	Port        int                          `json:"port"`
	SSL         *SSLConfig                   `json:"ssl"`
	PrettyJson  bool                         `json:"pretty_json"`
	Authorizers map[string]*AuthorizerConfig `json:"authorizers"`
	Endpoints   []*domain.EndpointConfig     `json:"endpoints"`
	ReloadEnv   bool                         `json:"reload_env"`
	Directory   string                       `json:"-"`
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
	if ok := AuthorizerExists(a.Type); !ok {
		err = fmt.Errorf("authorizer %s does not exist", a.Type)
	}

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
