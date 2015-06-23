package goexpose

import (
	"fmt"
	"net/http"

	"time"

	"io/ioutil"
	"runtime/debug"

	"code.google.com/p/gorilla/mux"
	"github.com/golang/glog"
)

var (
	version = "0.1"
)

/*
Returns new server instance
*/
func NewServer(config *Config) (server *Server, err error) {
	server = &Server{
		Config:  config,
		Version: version,
	}

	return
}

/*
Goexpose server
*/
type Server struct {

	// config instance
	Config *Config

	// Version
	Version string
}

/*
Runs http server
*/
func (s *Server) Run() (err error) {

	var router *mux.Router
	if router, err = s.router(); err != nil {
		return
	}

	// construct listen from host and port
	listen := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)

	// ssl version
	if s.Config.SSL != nil {
		glog.Infof("Start listen on https://%s", listen)
		if err = http.ListenAndServeTLS(listen, s.Config.SSL.Cert, s.Config.SSL.Key, router); err != nil {
			return
		}
	} else {
		glog.Infof("Start listen on http://%s", listen)
		if err = http.ListenAndServe(listen, router); err != nil {
			return
		}
	}

	return
}

/*
Creates new mux.Router registers all tasks to it and returns it.
*/
func (s *Server) router(ignored ...string) (router *mux.Router, err error) {
	// create new router
	router = mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(s.NotFoundHandler)

	var routes []*route
	if routes, err = s.routes(ignored...); err != nil {
		return
	}

	for _, route := range routes {
		// Log registered route
		glog.V(2).Infof("Register route for task: %s path: %s method: %v",
			route.TaskConfig.Type, route.Path, route.Method)

		// register route to router
		router.HandleFunc(route.Path, s.Handle(route.Task, route.Authorizers, route.EndpointConfig, route.TaskConfig)).Methods(route.Method)
	}

	return
}

/*
Route
*/
type route struct {
	Authorizers    Authorizers
	Method         string
	Path           string
	TaskConfig     *TaskConfig
	EndpointConfig *EndpointConfig
	Task           Tasker
}

/*
Returns prepared routes
*/
func (s *Server) routes(ignored ...string) (routes []*route, err error) {
	var (
		authorizers Authorizers
		factory     TaskFactory
		ok          bool
		tasks       []Tasker
	)

	routes = []*route{}
	// Get all authorizers
	if authorizers, err = GetAuthorizers(s.Config); err != nil {
		return
	}

Outer:
	for _, econfig := range s.Config.Endpoints {

		if err = econfig.Validate(); err != nil {
			return
		}

		for method, taskconf := range econfig.Methods {

			// ignored task
			for _, it := range ignored {
				if taskconf.Type == it {
					continue Outer
				}
			}

			// validate task config
			if err = taskconf.Validate(); err != nil {
				return
			}

			if factory, ok = getTaskFactory(taskconf.Type); !ok {
				err = fmt.Errorf("task %s doesn't exist", taskconf.Type)
				return
			}

			if tasks, err = factory(s, &taskconf); err != nil {
				err = fmt.Errorf("task %s returned error: %s", taskconf.Type, err)
				return
			}

			for _, task := range tasks {
				path := econfig.Path + task.Path()

				r := &route{
					Authorizers:    authorizers,
					EndpointConfig: econfig,
					Path:           path,
					Task:           task,
					TaskConfig:     &taskconf,
					Method:         method,
				}

				routes = append(routes, r)
			}

		}

	}
	return
}

/*
Handle func
*/
func (s *Server) Handle(task Tasker, authorizers Authorizers, ec *EndpointConfig, tc *TaskConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		defer func() {
			if e := recover(); e != nil {
				debug.PrintStack()
				NewResponse(http.StatusInternalServerError).Pretty(s.Config.PrettyJson).Error(e).Write(w, r, t)
			}
		}()

		// run authorizers on request
		if err := authorizers.Authorize(r, ec); err != nil {
			NewResponse(http.StatusUnauthorized).Write(w, r, t)
			return
		}

		// read request body
		var body = ""
		if r.Body != nil {
			if b, err := ioutil.ReadAll(r.Body); err != nil {
				body = string(b)
			}
		}

		/*
		 prepare data for task
		    mux vars are under "url"
		    cleaned query params are under "query"
		*/
		params := map[string]interface{}{
			"url":   mux.Vars(r),
			"query": s.GetQueryParams(r, ec),
			"request": map[string]interface{}{
				"method": r.Method,
				"body":   body,
			},
		}

		// prepare response
		response := NewResponse(http.StatusOK).Pretty(s.Config.PrettyJson)

		// should i add params
		if ec.QueryParams != nil {
			if ec.QueryParams.ReturnParams {
				response = response.AddValue("params", params)
			}
		}

		mqp := ec.Methods[r.Method].QueryParams
		if mqp != nil {
			if mqp.ReturnParams && !response.HasValue("params") {
				response = response.AddValue("params", params)
			}
		}

		if result, status, err := task.Run(r, params); err == nil {
			if status == 0 {
				status = http.StatusOK
			}
			response = response.Status(status).Result(result)
		} else {
			// @TODO: status wat?
			response.Status(http.StatusInternalServerError).Error(err)
			glog.Error(err)
		}

		response.Write(w, r, t)
	}
}

/*
Handler for not found
*/
func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	NewResponse(http.StatusNotFound).Write(w, r, t)
}

/*
Returns
*/
func (s *Server) GetQueryParams(r *http.Request, ec *EndpointConfig) (result map[string]string) {
	result = map[string]string{}
	if ec.QueryParams != nil {
		result = ec.QueryParams.GetParams(r)
		return
	}

	for k, v := range ec.Methods[r.Method].QueryParams.GetParams(r) {
		result[k] = v
	}

	return
}
