package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/auth"
	"github.com/phonkee/goexpose/internal/config"
	"github.com/phonkee/goexpose/internal/tasks/registry"
	"go.uber.org/zap"
	"io"
	"net/http"

	"runtime/debug"

	"os"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/phonkee/goexpose/internal/tasks"
)

var (
	Logo = `
     ______  ______  ______ __  __  ______ ______  ______  ______
    /\  ___\/\  __ \/\  ___/\_\_\_\/\  == /\  __ \/\  ___\/\  ___\
    \ \ \__ \ \ \/\ \ \  __\/_/\_\/\ \  _-\ \ \/\ \ \___  \ \  __\
     \ \_____\ \_____\ \_____/\_\/\_\ \_\  \ \_____\/\_____\ \_____\
      \/_____/\/_____/\/_____\/_/\/_/\/_/   \/_____/\/_____/\/_____/
                                                            version: %v`
)

// New returns new server instance
func New(config *config.Config) (server *Server, err error) {
	server = &Server{
		Config: config,
		//Version: goexpose.VERSION,
	}

	return
}

// Server is goexpose server
type Server struct {

	// config instance
	Config *config.Config

	// Version
	Version string

	// Router
	Router *mux.Router
}

// Run runs http server
func (s *Server) Run(ctx context.Context) (err error) {

	zap.L().Info("starting server")

	if s.Router, err = s.router(); err != nil {
		return
	}

	// construct listen from host and port
	listen := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)

	zap.L().Info("listening", zap.String("address", listen))

	// ssl version
	if s.Config.SSL != nil {
		if err = http.ListenAndServeTLS(listen, s.Config.SSL.Cert, s.Config.SSL.Key, s.Router); err != nil {
			return
		}
	} else {
		if err = http.ListenAndServe(listen, s.Router); err != nil {
			return
		}
	}

	return
}

func (s *Server) GetVersion() string {
	return s.Version
}

/*
Creates new mux.Router registers all tasks to it and returns it.
*/
func (s *Server) router(ignored ...string) (router *mux.Router, err error) {
	// create new router
	router = mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(s.NotFoundHandler)

	var routes []*domain.Route
	if routes, err = s.GetRoutes(ignored); err != nil {
		return
	}

	for _, route := range routes {
		// Log registered route
		zap.L().Info("register route",
			zap.String("path", route.Path),
			zap.String("method", route.Method),
			zap.String("task", route.TaskConfig.Type),
		)

		// register route to router
		router.HandleFunc(route.Path, s.Handle(route.Task, route.Authorizers, route.EndpointConfig, route.TaskConfig)).Methods(route.Method).Name(route.EndpointConfig.RouteName())
	}

	return
}

/*
Route
*/
type route struct {
	Authorizers    domain.Authorizers
	Method         string
	Path           string
	TaskConfig     *domain.TaskConfig
	EndpointConfig *domain.EndpointConfig
	Task           domain.Task
}

// GetRoutes Returns prepared routes
func (s *Server) GetRoutes(ignored []string) (routes []*domain.Route, err error) {
	var (
		authorizers domain.Authorizers
		factory     domain.TaskInitFunc
		ok          bool
		tasks       []domain.Task
	)

	routes = []*domain.Route{}
	// Get all authorizers
	if authorizers, err = auth.GetAuthorizers(s.Config); err != nil {
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

			if factory, ok = registry.GetTaskInitFunc(taskconf.Type); !ok {
				err = fmt.Errorf("task %s doesn't exist", taskconf.Type)
				return
			}

			if tasks, err = factory(s, &taskconf, econfig); err != nil {
				err = fmt.Errorf("task %s returned error: %s", taskconf.Type, err)
				return
			}

			for _, task := range tasks {
				path := econfig.Path + task.Path()

				r := &domain.Route{
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
func (s *Server) Handle(task domain.Task, authorizers domain.Authorizers, ec *domain.EndpointConfig, tc *domain.TaskConfig) http.HandlerFunc {

	env := s.GetEnv()

	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				debug.PrintStack()
				response.New(http.StatusInternalServerError).Error(e).Write(w, r)
			}
		}()

		// run authorizers on request
		if err := authorizers.Authorize(r, ec); err != nil {
			response.New(http.StatusUnauthorized).Write(w, r)
			return
		}

		// read request body
		var body = ""
		if r.Body != nil {
			if b, err := io.ReadAll(r.Body); err != nil {
				body = string(b)
			}
		}

		/*
		 prepare data for task
		    mux vars are under "url"
		    cleaned query params are under "query"
		*/
		requestData := map[string]interface{}{
			"method": r.Method,
			"body":   body,
		}

		// if we have json content type, try to parse it and add it to request data
		if r.Header.Get("Content-Type") == "application/json" {
			jsonData := make(map[string]interface{})
			if err := json.Unmarshal([]byte(body), &jsonData); err == nil {
				requestData["json"] = jsonData
			}
		}

		params := map[string]interface{}{
			"url":     mux.Vars(r),
			"query":   s.GetQueryParams(r, ec),
			"request": requestData,
		}

		// reload environment every request?
		if s.Config.ReloadEnv {
			params["env"] = s.GetEnv()
		} else {
			params["env"] = env
		}

		// prepare response
		resp := task.Run(r, params)

		// should i add params
		if ec.QueryParams != nil {
			if ec.QueryParams.ReturnParams {
				resp = resp.Data("params", params)
			}
		}

		mqp := ec.Methods[r.Method].QueryParams
		if mqp != nil {
			if mqp.ReturnParams && !resp.HasData("params") {
				resp = response.Data("params", params)
			}
		}

		resp.Write(w, r)
	}
}

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	response.NotFound().Write(w, r)
}

func (s *Server) GetQueryParams(r *http.Request, ec *domain.EndpointConfig) (result map[string]string) {
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
