package goexpose

import "net/http"

/*
TaskFactory returns instance of task by server and config
*/
type TaskFactory func(server *Server, config *TaskConfig, ec *EndpointConfig) ([]Tasker, error)

/*
Tasker interface
Main task
*/
type Tasker interface {
	// Returns path for task
	Path() string

	// Run method is called on http request
	Run(r *http.Request, vars map[string]interface{}) *Response
}

/*
Base task
*/
type Task struct{}

/*
Default path is blank
*/
func (t *Task) Path() string {
	return ""
}
