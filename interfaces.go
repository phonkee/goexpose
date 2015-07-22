package goexpose

import (
	"net/http"
	"bytes"
	"text/template"
)

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

/*
Interpolates string as text template with data
 */
func (t *Task) Interpolate(tpl string, data interface{}) (result string, err error) {
	// prepare buffer
	b := bytes.NewBuffer([]byte{})

	// interpolate url
	tmpl, _ := template.New("new").Parse(tpl)
	if err = tmpl.Execute(b, data); err != nil {
		return
	}

	return b.String(), nil
}
