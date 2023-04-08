package config

import (
	"encoding/json"
	"github.com/ghodss/yaml"
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
