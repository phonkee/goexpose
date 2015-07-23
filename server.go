package goexpose

import (
	"fmt"
	"net/http"

	"time"

	"io/ioutil"
	"runtime/debug"

	"os"
	"strings"

	"code.google.com/p/gorilla/mux"
	"github.com/golang/glog"
)

var (
	version = "0.2"
	logo    = `
     ______  ______  ______ __  __  ______ ______  ______  ______
    /\  ___\/\  __ \/\  ___/\_\_\_\/\  == /\  __ \/\  ___\/\  ___\
    \ \ \__ \ \ \/\ \ \  __\/_/\_\/\ \  _-\ \ \/\ \ \___  \ \  __\
     \ \_____\ \_____\ \_____/\_\/\_\ \_\  \ \_____\/\_____\ \_____\
      \/_____/\/_____/\/_____\/_/\/_/\/_/   \/_____/\/_____/\/_____/
                                                              v ` + version
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

	// Router
	Router *mux.Router
}

/*
Runs http server
*/
func (s *Server) Run() (err error) {

	glog.V(2).Info(logo)

	if s.Router, err = s.router(); err != nil {
		return
	}

	// construct listen from host and port
	listen := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)

	// ssl version
	if s.Config.SSL != nil {
		glog.Infof("Start listen on https://%s", listen)
		if err = http.ListenAndServeTLS(listen, s.Config.SSL.Cert, s.Config.SSL.Key, s.Router); err != nil {
			return
		}
	} else {
		glog.Infof("Start listen on http://%s", listen)
		if err = http.ListenAndServe(listen, s.Router); err != nil {
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
		router.HandleFunc(route.Path, s.Handle(route.Task, route.Authorizers, route.EndpointConfig, route.TaskConfig)).Methods(route.Method).Name(route.EndpointConfig.RouteName())
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

			if tasks, err = factory(s, &taskconf, econfig); err != nil {
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

	env := s.GetEnv()

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

		// reload environment every request?
		if s.Config.ReloadEnv {
			params["env"] = s.GetEnv()
		} else {
			params["env"] = env
		}

		// prepare response
		response := task.Run(r, params)

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

/*
Get environment variables
*/
func (s *Server) GetEnv() map[string]interface{} {
	result := map[string]interface{}{}
	for _, v := range os.Environ() {
		splitted := strings.SplitN(v, "=", 2)
		if len(splitted) != 2 {
			continue
		}

		key, value := strings.TrimSpace(splitted[0]), strings.TrimSpace(splitted[1])
		result[key] = value
	}
	return result
}
