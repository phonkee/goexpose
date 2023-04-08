package config

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/phonkee/goexpose/domain"
	"sync"
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

func unmarshalConfig(content, extension string) (*Config, error) {
	configFormatsLock.RLock()
	defer configFormatsLock.RUnlock()

	uf, ok := configFormats[extension]
	if !ok {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidConfigType, extension)
	}

	config := NewConfig()
	if err := uf([]byte(content), config); err != nil {
		return nil, err
	}
	return config, nil
}
