package domain

import (
	"github.com/phonkee/go-response"
	"net/http"
)

// TaskInitFunc returns instance of task by server and config
type TaskInitFunc func(server Server, config *TaskConfig, ec *EndpointConfig) ([]Task, error)

// Task interface
type Task interface {
	// Path returns path for task
	Path() string

	// Run method is called on http request
	Run(r *http.Request, vars map[string]interface{}) response.Response
}

// BaseTask is task with default values
type BaseTask struct{}

func (t BaseTask) Path() string {
	return ""
}
