package goexpose

import (
	"encoding/json"
	"net/http"

	"fmt"
	"os/exec"

	"io"
	"io/ioutil"
	"strings"

	"github.com/garyburd/redigo/redis"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func init() {
	// register task factories
	RegisterTaskFactory("shell", ShellTaskFactory)
	RegisterTaskFactory("info", InfoTaskFactory)
	RegisterTaskFactory("http", HttpTaskFactory)
	RegisterTaskFactory("postgres", PostgresTaskFactory)
	RegisterTaskFactory("redis", RedisTaskFactory)
}

/*
Config for info task
*/
type ShellTaskConfig struct {
	Env      map[string]string `json:"env"`
	Shell    string            `json:"shell"`
	Commands []struct {
		Command       string `json:"command`
		Chdir         string `json:"chdir"`
		Format        string `json:"format"`
		ReturnCommand bool   `json:"return_command"`
	} `json:"commands"`
}

func NewShellTaskConfig() *ShellTaskConfig {
	return &ShellTaskConfig{
		Shell: "/bin/sh",
		Env:   map[string]string{},
	}
}

/*
Factory for SHellTask task
*/
func ShellTaskFactory(server *Server, taskconfig *TaskConfig) (tasks []Tasker, err error) {
	config := NewShellTaskConfig()
	if err = json.Unmarshal(taskconfig.Config, config); err != nil {
		return
	}

	tasks = []Tasker{&ShellTask{
		Config: config,
	}}
	return
}

/*
ShellTask runs shel commands
*/
type ShellTask struct {
	Task

	// config
	Config *ShellTaskConfig
}

/*
Run method for shell task
Run all commands and return results
*/
func (s *ShellTask) Run(r *http.Request, data map[string]interface{}) (interface{}, int, error) {

	results := []map[string]interface{}{}

	// run all commands
	for _, command := range s.Config.Commands {

		cmdresult := map[string]interface{}{
			"out":   nil,
			"error": nil,
		}

		var (
			b string
			e error
		)
		if b, e = s.Interpolate(command.Command, data); e != nil {
			cmdresult["error"] = e
			results = append(results, cmdresult)
			continue
		}

		finalCommand := b

		// show command in result
		if command.ReturnCommand {
			cmdresult["command"] = finalCommand
		}

		// run command
		cmd := exec.Command(s.Config.Shell, "-c", finalCommand)

		// change directory if needed
		if command.Chdir != "" {
			cmd.Dir = command.Chdir
		}

		// add env vars
		for k, v := range s.Config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// get output
		if out, err := cmd.Output(); err != nil {
			cmdresult["error"] = err
			results = append(results, cmdresult)
			continue
		} else {
			cmdresult["out"] = strings.TrimSpace(string(out))
			results = append(results, cmdresult)
		}
	}

	final := map[string]interface{}{
		"commands": results,
	}
	return final, http.StatusOK, nil
}

/*
Factory for InfoTask task
*/
func InfoTaskFactory(server *Server, taskconfig *TaskConfig) (tasks []Tasker, err error) {

	// get information about all routes
	var routes []*route
	if routes, err = server.routes("info"); err != nil {
		return
	}

	tasks = []Tasker{&InfoTask{
		version: server.Version,
		routes:  routes,
	}}
	return
}

/*
InfoTask - information about goexpose server
*/
type InfoTask struct {
	Task

	// store version
	version string

	// all other routes
	routes []*route
}

/*
InfoTask Run method.
*/
func (i *InfoTask) Run(r *http.Request, data map[string]interface{}) (interface{}, int, error) {
	data = map[string]interface{}{
		"version": i.version,
	}

	type Item struct {
		Path        string   `json:"path"`
		Method      string   `json:"method,omitempty"`
		Authorizers []string `json:"authorizers,omitempty"`
		Type        string   `json:"type"`
	}

	routes := []Item{}

	// add tasks to result
	for _, route := range i.routes {
		item := Item{
			Path:        route.Path,
			Method:      route.Method,
			Authorizers: route.TaskConfig.Authorizers,
			Type:        route.TaskConfig.Type,
		}

		routes = append(routes, item)
	}
	data["tasks"] = routes

	return data, http.StatusOK, nil
}

/*
HttpTask configuration

Attrs:
Method - if blank, method from request will be used
Format - "json", "text", ""
	if blank json will be guessed from Content-Type header
*/

type HttpTaskConfig struct {
	URLs []*HttpTaskConfigURL `json:"urls"`
}

type HttpTaskConfigURL struct {
	URL           string `json:"url"`
	Method        string `json:"method"`
	PostBody      bool   `json:"post_body"`
	Format        string `json:"format"`
	ReturnHeaders bool   `json:"return_headers`
}

/*
Validate config
*/
func (h *HttpTaskConfig) Validate() (err error) {
	for _, url := range h.URLs {
		url.URL = strings.TrimSpace(url.URL)
		if url.URL == "" {
			return fmt.Errorf("Invalid url in http task.")
		}

		switch url.Format {
		case "text", "json", "":
			// no-op
		default:
			return fmt.Errorf("unknown format %s", url.Format)
		}
	}

	return
}

/*
HttpTaskFactory - factory to create HttpTasks
*/
func HttpTaskFactory(server *Server, tc *TaskConfig) (tasks []Tasker, err error) {
	// default config
	config := &HttpTaskConfig{}
	if err = config.Validate(); err != nil {
		return
	}

	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	// return tasks
	tasks = []Tasker{&HttpTask{
		config: config,
	}}
	return
}

/*
HttpTask
	task that can make requests to given urls
*/
type HttpTask struct {
	Task

	// http configuration
	config *HttpTaskConfig
}

/*
Run method is called on request
@TODO: please refactor me!
*/
func (h *HttpTask) Run(r *http.Request, data map[string]interface{}) (rr interface{}, status int, err error) {

	results := []map[string]interface{}{}

	status = http.StatusOK

	for _, url := range h.config.URLs {

		client := &http.Client{}
		var req *http.Request

		var body io.Reader

		// prepare response
		result := map[string]interface{}{}

		if url.PostBody {
			body = r.Body
		}

		method := r.Method

		// if method is given
		if url.Method != "" {
			method = url.Method
		}

		var b string
		if b, err = h.Interpolate(url.URL, data); err != nil {
			result["error"] = err.Error()
			results = append(results, result)
			continue
		}

		if req, err = http.NewRequest(method, b, body); err != nil {
			result["error"] = err.Error()
			results = append(results, result)
			continue
		}

		var resp *http.Response
		if resp, err = client.Do(req); err != nil {
			result["error"] = err.Error()
			results = append(results, result)
			continue
		}

		var respbody []byte
		if respbody, err = ioutil.ReadAll(resp.Body); err != nil {
			result["error"] = err.Error()
			results = append(results, result)
			continue
		}

		// prepare response
		result["status"] = resp.StatusCode

		// return headers?
		if url.ReturnHeaders {
			result["headers"] = resp.Header
		}

		// get format(if available)
		format := url.Format

		// try to guess json
		if format == "" {
			ct := strings.ToLower(r.Header.Get("Content-Type"))
			if strings.Contains(ct, "application/json") {
				format = "json"
			} else {
				format = "text"
			}
		}

		// examine formats
	Out:
		switch format {
		case "json":
			rdata := map[string]interface{}{}
			if err = json.Unmarshal(respbody, &rdata); err != nil {
				format = "text"
				break Out
			}
			result["response"] = rdata
			result["format"] = format
		case "text":
			result["response"] = string(respbody)
			result["format"] = format
		}

		results = append(results, result)
	}

	rrr := map[string]interface{}{
		"urls": results,
	}
	return rrr, status, nil
}

/*
PostgresTask

run queries on postgres database
*/

func PostgresTaskFactory(server *Server, tc *TaskConfig) (tasks []Tasker, err error) {
	config := &PostgresTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}
	tasks = []Tasker{&PostgresTask{
		config: config,
	}}

	return
}

type PostgresTaskConfig struct {
	URL           string                     `json:"url"`
	Queries       []*PostgresTaskConfigQuery `json:"queries"`
	ReturnQueries bool                       `json:"return_queries"`
}

type PostgresTaskConfigQuery struct {
	Query string   `json:"query"`
	Args  []string `json:"args"`
}

/*
Postgres task
*/
type PostgresTask struct {
	Task

	// configuration
	config *PostgresTaskConfig
}

/*
Run postgres task
*/
func (p *PostgresTask) Run(r *http.Request, data map[string]interface{}) (interface{}, int, error) {

	type Item struct {
		Error string                   `json:"error,omitempty"`
		Rows  []map[string]interface{} `json:"rows,omitempty"`
		Query string                   `json:"query,omitempty"`
		Args  []interface{}            `json:"args,omitempty"`
	}
	queryresults := []Item{}
Outer:
	for _, query := range p.config.Queries {

		item := Item{}
		// interpolate all args
		args := []interface{}{}
		for _, arg := range query.Args {
			interpolated, e := p.Interpolate(arg, data)
			if e != nil {
				item.Error = e.Error()
				queryresults = append(queryresults, item)
				continue
			}
			args = append(args, interpolated)
		}

		// add query with args to response?
		if p.config.ReturnQueries {
			item.Query = query.Query
			item.Args = args
		}

		db, err := sqlx.Connect("postgres", p.config.URL)
		if err != nil {
			item.Error = err.Error()
			queryresults = append(queryresults, item)
			continue
		}

		// run query
		rows, err := db.Queryx(query.Query, args...)
		if err != nil {
			item.Error = err.Error()
			queryresults = append(queryresults, item)
			continue Outer
		}

		for rows.Next() {
			results := make(map[string]interface{})
			err = rows.MapScan(results)
			if err != nil {
				item.Error = err.Error()
				queryresults = append(queryresults, item)
				continue Outer
			}
			item.Rows = append(item.Rows, results)
		}

		queryresults = append(queryresults, item)
	}

	// final result
	result := map[string]interface{}{
		"queries": queryresults,
	}

	return result, http.StatusOK, nil

}

/*
RedisTask

task to call commands on redis
*/

type RedisTaskConfig struct {
	Address       string                 `json:"address"`
	Database      int                    `json:"database"`
	Network       string                 `json:"network"`
	Queries       []RedisTaskConfigQuery `json:"queries"`
	ReturnQueries bool                   `json:"return_queries"`
}

func (r *RedisTaskConfig) Validate() (err error) {
	for _, i := range r.Queries {
		if err = i.Validate(); err != nil {
			return
		}
	}

	return
}

type RedisTaskConfigQuery struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Type    string   `json:"type"`
}

var (
	redisunderstands = map[string]func(interface{}, error) (interface{}, error){
		"bool": func(r interface{}, e error) (interface{}, error) {
			return redis.Bool(r, e)
		},
		"float64": func(r interface{}, e error) (interface{}, error) {
			return redis.Float64(r, e)
		},
		"int": func(r interface{}, e error) (interface{}, error) {
			return redis.Int(r, e)
		},
		"int64": func(r interface{}, e error) (interface{}, error) {
			return redis.Int64(r, e)
		},
		"ints": func(r interface{}, e error) (interface{}, error) {
			return redis.Ints(r, e)
		},
		"string": func(r interface{}, e error) (interface{}, error) {
			return redis.String(r, e)
		},
		"strings": func(r interface{}, e error) (interface{}, error) {
			return redis.Strings(r, e)
		},
		"uint64": func(r interface{}, e error) (interface{}, error) {
			return redis.Uint64(r, e)
		},
		"values": func(r interface{}, e error) (interface{}, error) {
			return redis.Values(r, e)
		},
		"stringmap": func(r interface{}, e error) (interface{}, error) {
			return redis.StringMap(r, e)
		},
	}
)

func (r *RedisTaskConfigQuery) Validate() (err error) {
	for ru, _ := range redisunderstands {
		if r.Type == ru {
			return nil
		}
	}
	return fmt.Errorf("unknown redis type %s", r.Type)
}

/*
Factory to create task instances
*/
func RedisTaskFactory(server *Server, tc *TaskConfig) (result []Tasker, err error) {
	// address defaults to tcp
	config := &RedisTaskConfig{
		Address:  ":6379",
		Network:  "tcp",
		Database: 1,
	}

	// validate config
	if err = config.Validate(); err != nil {
		return
	}

	// unmarshall config
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}
	result = []Tasker{
		&RedisTask{
			config: config,
		},
	}
	return
}

type RedisTask struct {
	Task

	// config instance
	config *RedisTaskConfig
}

/*
Run method runs when request comes...
*/
func (rt *RedisTask) Run(r *http.Request, data map[string]interface{}) (rr interface{}, status int, err error) {

	type Item struct {
		Error   string        `json:"error,omitempty"`
		Result  interface{}   `json:"result,omitempty"`
		Command string        `json:"command,omitempty"`
		Args    []interface{} `json:"args,omitempty"`
	}

	status = http.StatusOK

	var conn redis.Conn
	if conn, err = redis.Dial(rt.config.Network, rt.config.Address); err != nil {
		return
	}

	queries := []Item{}

	var (
		reply interface{}
		grr   interface{}
	)
	for _, query := range rt.config.Queries {
		item := Item{}
		args := []interface{}{}
		for _, arg := range query.Args {
			var ia string
			if ia, err = rt.Interpolate(arg, data); err != nil {
				item.Error = err.Error()
				goto AddItem
			}
			args = append(args, ia)
		}

		// return full query?
		if rt.config.ReturnQueries {
			item.Command = query.Command
			item.Args = args
		}

		if reply, err = conn.Do(query.Command, args...); err != nil {
			item.Error = err.Error()
			goto AddItem
		}

		// not found (not nice but..)
		if reply == nil {
			item.Error = "404"
			goto AddItem
		}

		if grr, err = rt.GetReply(reply, query); err != nil {
			item.Error = err.Error()
			goto AddItem
		}

		item.Result = grr

	AddItem:
		queries = append(queries, item)
	}

	result := map[string]interface{}{
		"queries": queries,
	}
	return result, status, nil
}

func (r *RedisTask) GetReply(reply interface{}, query RedisTaskConfigQuery) (interface{}, error) {
	if fn, ok := redisunderstands[query.Type]; !ok {
		return nil, fmt.Errorf("unknown redis type %s", query.Type)
	} else {
		return fn(reply, nil)
	}

}
