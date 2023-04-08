package config

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
)

type Logging struct {
	Level    string `json:"level" default:"debug"`
	Stdout   bool   `json:"stdout" default:"false"`
	Filename string `json:"filename" default:""`
}

func (l *Logging) Validate() error {
	return nil
}

func (l *Logging) Setup() error {
	cfg := zap.NewProductionConfig()

	// set message key
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if err := cfg.Level.UnmarshalText([]byte(l.Level)); err != nil {
		return fmt.Errorf("invalid logging level: %w", err)
	}

	if cfg.Level.Level() == zapcore.DebugLevel {
		cfg.DisableCaller = true
	}

	// cleanup output paths
	cfg.OutputPaths = []string{}

	// add file logging to output path
	if filename := strings.TrimSpace(l.Filename); filename != "" {
		cfg.OutputPaths = append(cfg.OutputPaths, filename)
	}

	// add stdout logging to output path
	if l.Stdout {
		cfg.OutputPaths = append(cfg.OutputPaths, "stdout")
	}

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
