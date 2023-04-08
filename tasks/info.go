package tasks

import (
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/tasks/registry"
	"net/http"
)

func init() {
	registry.RegisterTaskInitFunc("info", InfoTaskFactory)
}

// InfoTaskFactory is factory for InfoTask task (million dollar comment)
func InfoTaskFactory(server domain.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {

	// get information about all routes
	var routes []*domain.Route
	if routes, err = server.GetRoutes([]string{"info"}); err != nil {
		return
	}

	tasks = []domain.Task{&InfoTask{
		version: server.GetVersion(),
		routes:  routes,
	}}
	return
}

/*
InfoTask - information about goexpose server
*/
type InfoTask struct {
	domain.BaseTask

	// store version
	version string

	// all other routes
	routes []*domain.Route
}

type TaskInfo struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Authorizers []string `json:"authorizers,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
}

// Run run run.
func (i *InfoTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	endpoints := make([]TaskInfo, 0)

	// add tasks to result
	for _, route := range i.routes {
		endpoints = append(endpoints, TaskInfo{
			Path:        route.Path,
			Method:      route.Method,
			Authorizers: route.TaskConfig.Authorizers,
			Description: route.TaskConfig.Description,
			Type:        route.TaskConfig.Type,
		})
	}

	return response.OK().Result(map[string]interface{}{
		"version":   i.version,
		"endpoints": endpoints,
	})
}
