package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// NewFromFilename returns config from filename
// Currently supported formats are json and yaml.
func NewFromFilename(filename string) (config *Config, err error) {
	config = NewConfig()

	var (
		contents []byte
	)
	if contents, err = os.ReadFile(filename); err != nil {
		return
	}

	var format string
	ext := strings.ToLower(strings.TrimLeft(filepath.Ext(filename), "."))

	// unmarshal config
	if config, err = unmarshalConfig(string(contents), ext); err != nil {
		return
	}

	found := false
	for name, fmtUnmarshalFunc := range configFormats {
		if name == format {
			if err = fmtUnmarshalFunc(contents, config); err != nil {
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

	//// unmarshal config
	//if err = json.Unmarshal(contents, config); err != nil {
	//	return
	//}

	// get config dir
	if config.Directory, err = filepath.Abs(filepath.Dir(filename)); err != nil {
		return
	}
	// and raw config for some special cases
	config.Raw = contents

	return
}
