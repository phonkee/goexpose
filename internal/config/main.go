package config

import (
	"os"
	"path/filepath"
	"strings"
)

// NewFromFilename returns config from filename
// Currently supported formats are json and yaml.
func NewFromFilename(filename string) (config *Config, err error) {
	config = NewConfig()
	config.Filename = filename

	var (
		contents []byte
	)
	if contents, err = os.ReadFile(filename); err != nil {
		return
	}

	ext := strings.ToLower(strings.TrimLeft(filepath.Ext(filename), "."))

	// unmarshal config
	if config, err = unmarshalConfig(string(contents), ext); err != nil {
		return
	}

	// get config dir
	if config.Directory, err = filepath.Abs(filepath.Dir(filename)); err != nil {
		return
	}
	// and raw config for some special cases
	config.Raw = contents

	return
}
