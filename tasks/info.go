package tasks

import (
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"net/http"
)

func init() {
	goexpose.RegisterTaskFactory("info", InfoTaskFactory)
}

/*
Factory for InfoTask task
*/
func InfoTaskFactory(server domain.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {

	// get information about all routes
	var routes []*domain.Route
	if routes, err = server.routes("info"); err != nil {
		return
	}

	tasks = []domain.Task{&InfoTask{
		version: server.Version(),
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

/*
InfoTask Run method.
*/
func (i *InfoTask) Run(r *http.Request, data map[string]interface{}) (response domain.Response) {

	endpoints := []*goexpose.Response{}

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

	return goexpose.NewResponse(http.StatusOK).Result(map[string]interface{}{
		"version":   i.version,
		"endpoints": endpoints,
	})
}
