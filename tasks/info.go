package tasks

import (
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"net/http"
)

func init() {
	goexpose.RegisterTaskFactory("info", InfoTaskFactory)
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

// Run run run.
func (i *InfoTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	endpoints := make([]*goexpose.Response, 0)

	// add tasks to result
	for _, route := range i.routes {
		r := goexpose.NewResponse(http.StatusOK)
		r.AddValue("path", route.Path)
		r.AddValue("method", route.Method)
		if len(route.TaskConfig.Authorizers) > 0 {
			r.AddValue("authorizers", route.TaskConfig.Authorizers)
		}
		r.AddValue("type", route.TaskConfig.Type)
		if route.TaskConfig.Description != "" {
			r.AddValue("description", route.TaskConfig.Description)
		}
		endpoints = append(endpoints, r.StripStatusData())
	}

	return response.OK().Result(map[string]interface{}{
		"version":   i.version,
		"endpoints": endpoints,
	})
}
