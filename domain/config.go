package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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
