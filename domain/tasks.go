package domain

import "net/http"

/*
TaskFactory returns instance of task by server and config
*/
type TaskFactory func(server Server, config *TaskConfig, ec *EndpointConfig) ([]Tasker, error)

/*
Tasker interface
Main task
*/
type Tasker interface {
	// Path returns path for task
	Path() string

	// Run method is called on http request
	Run(r *http.Request, vars map[string]interface{}) Response
}

// BaseTask is base task
type BaseTask struct{}

func (t BaseTask) Path() string {
	return ""
}
