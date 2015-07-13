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
	"github.com/go-sql-driver/mysql"
	"github.com/gocql/gocql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func init() {
	// register task factories
	RegisterTaskFactory("cassandra", CassandraTaskFactory)
	RegisterTaskFactory("http", HttpTaskFactory)
	RegisterTaskFactory("info", InfoTaskFactory)
	RegisterTaskFactory("mysql", MySQLTaskFactory)
	RegisterTaskFactory("postgres", PostgresTaskFactory)
	RegisterTaskFactory("redis", RedisTaskFactory)
	RegisterTaskFactory("shell", ShellTaskFactory)
}

/*
Config for info task
*/
type ShellTaskConfig struct {
	Env      map[string]string         `json:"env"`
	Shell    string                    `json:"shell"`
	Commands []*ShellTaskConfigCommand `json:"commands"`
}

func (s *ShellTaskConfig) Validate() (err error) {
	for _, c := range s.Commands {
		if err = c.Validate(); err != nil {
			return
		}
	}
	return
}

type ShellTaskConfigCommand struct {
	Command       string `json:"command"`
	Chdir         string `json:"chdir"`
	Format        string `json:"format"`
	ReturnCommand bool   `json:"return_command"`
}

func (s *ShellTaskConfigCommand) Validate() (err error) {
	if s.Format, err = VerifyFormat(s.Format); err != nil {
		return
	}
	return
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

	if err = config.Validate(); err != nil {
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

			// format out
			if re, f, e := Format(string(strings.TrimSpace(string(out))), command.Format); e == nil {
				cmdresult["out"] = re
				cmdresult["format"] = f
			} else {
				cmdresult["error"] = e
			}

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
		Description string   `json:"description,omitempty"`
	}

	endpoints := []Item{}

	// add tasks to result
	for _, route := range i.routes {
		item := Item{
			Path:        route.Path,
			Method:      route.Method,
			Authorizers: route.TaskConfig.Authorizers,
			Type:        route.TaskConfig.Type,
			Description: route.TaskConfig.Description,
		}

		endpoints = append(endpoints, item)
	}
	data["endpoints"] = endpoints

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
	ReturnHeaders bool   `json:"return_headers"`
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

		if url.Format, err = VerifyFormat(url.Format); err != nil {
			return err
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

		if url.PostBody && r.Body != nil {
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
		if !HasFormat(format, "json") {
			ct := strings.ToLower(r.Header.Get("Content-Type"))
			if strings.Contains(ct, "application/json") {
				if !HasFormat(format, "json") {
					format = AddFormat(format, "json")
				}
			}
		}

		if re, f, e := Format(string(respbody), url.Format); e == nil {
			result["response"] = re
			result["format"] = f
		} else {
			result["error"] = e
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
	Queries       []*PostgresTaskConfigQuery `json:"queries"`
	ReturnQueries bool                       `json:"return_queries"`
}

type PostgresTaskConfigQuery struct {
	URL   string   `json:"url"`
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
		Error     string                   `json:"error,omitempty"`
		ErrorCode string                   `json:"error_code,omitempty"`
		Rows      []map[string]interface{} `json:"rows,omitempty"`
		Query     string                   `json:"query,omitempty"`
		Args      []interface{}            `json:"args,omitempty"`
	}
	queryresults := []Item{}

	for _, query := range p.config.Queries {

		item := Item{}
		var (
			err error
			url string
		)
		if url, err = p.Interpolate(query.URL, data); err != nil {
			item.Error = err.Error()
			queryresults = append(queryresults, item)
			continue
		}

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

		var (
			rows *sqlx.Rows
			errq error
		)

		db, err := sqlx.Connect("postgres", url)
		if err != nil {

			if err, ok := err.(*pq.Error); ok {
				item.ErrorCode = err.Code.Name()
			}

			item.Error = err.Error()
			goto Append
		}

		// run query
		rows, errq = db.Queryx(query.Query, args...)
		if errq != nil {
			if errq, ok := errq.(*pq.Error); ok {
				item.ErrorCode = errq.Code.Name()
			}

			item.Error = errq.Error()
			goto Append
		}

		for rows.Next() {
			results := make(map[string]interface{})
			err = rows.MapScan(results)
			if err != nil {
				if err, ok := err.(*pq.Error); ok {
					item.ErrorCode = err.Code.Name()
				}

				item.Error = err.Error()
				goto Append
			}
			item.Rows = append(item.Rows, results)
		}

	Append:
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

	var address string
	if address, err = rt.Interpolate(rt.config.Address, data); err != nil {
		return
	}

	var conn redis.Conn
	if conn, err = redis.Dial(rt.config.Network, address); err != nil {
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

/*
Cassandra Task

Run queries on cassandra cluster
*/

type CassandraTaskConfig struct {
	Queries       []CassandraTaskConfigQuery `json:"queries"`
	ReturnQueries bool                       `json:"return_queries"`
}

/*
Validate config
*/
func (c *CassandraTaskConfig) Validate() (err error) {

	if len(c.Queries) == 0 {
		return fmt.Errorf("please provide at least one cassandra query")
	}
	for _, query := range c.Queries {
		if err = query.Validate(); err != nil {
			return err
		}
	}

	return
}

/*
Config for Query
*/
type CassandraTaskConfigQuery struct {
	Cluster  []string `json:"cluster"`
	Keyspace string   `json:"keyspace"`
	Query    string   `json:"query"`
	Args     []string `json:"args"`
}

/*
Validate query config
*/
func (c *CassandraTaskConfigQuery) Validate() (err error) {
	if len(c.Cluster) == 0 {
		return fmt.Errorf("cluster must have at least one url")
	}

	c.Keyspace = strings.TrimSpace(c.Keyspace)
	if c.Keyspace == "" {
		return fmt.Errorf("please provide keyspace.")
	}

	return
}

func CassandraTaskFactory(s *Server, tc *TaskConfig) (result []Tasker, err error) {
	config := &CassandraTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return

	}
	if err = config.Validate(); err != nil {
		return
	}
	result = []Tasker{&CassandraTask{
		config: config,
	}}
	return
}

/*
Cassandra task to run queries on cassandra
*/
type CassandraTask struct {
	Task

	// configuration
	config *CassandraTaskConfig
}

/*
Run cassandra task
*/
func (c *CassandraTask) Run(r *http.Request, data map[string]interface{}) (res interface{}, status int, err error) {

	type ResultQuery struct {
		Query  string                   `json:"query,omitempty"`
		Args   []interface{}            `json:"args,omitempty"`
		Error  error                    `json:"error"`
		Result []map[string]interface{} `json:"result"`
	}

	type Result struct {
		Queries []ResultQuery `json:"queries"`
	}

	result := &Result{
		Queries: []ResultQuery{},
	}

	for _, query := range c.config.Queries {
		args := []interface{}{}

		rquery := ResultQuery{
			Result: []map[string]interface{}{},
		}

		chosts := []string{}
		for _, i := range query.Cluster {
			var chost string
			if chost, err = c.Interpolate(i, data); err != nil {
				return
			}
			chosts = append(chosts, chost)
		}

		// instantiate cluster
		cluster := gocql.NewCluster(chosts...)
		if cluster.Keyspace, err = c.Interpolate(query.Keyspace, data); err != nil {
			return
		}

		session, err := cluster.CreateSession()
		if err != nil {
			rquery.Error = err
			goto Append
		}

		if c.config.ReturnQueries {
			rquery.Query = query.Query
		}

		for _, arg := range query.Args {
			final, err := c.Interpolate(arg, data)
			if err != nil {
				rquery.Error = err
				goto Append
			} else {
				args = append(args, final)
			}
		}

		if c.config.ReturnQueries {
			rquery.Args = args
		}

		// slicemap to result
		if rquery.Result, err = session.Query(query.Query, args...).Iter().SliceMap(); err != nil {
			rquery.Error = err
			goto Append
		} else {
			goto Append
		}

	Append:
		result.Queries = append(result.Queries, rquery)
	}
	res = result

	return
}

/*
MySQLTask

run queries on mysql
*/

type MySQLTaskConfig struct {
	ReturnQueries bool                    `json:"return_queries"`
	Queries       []*MySQLTaskConfigQuery `json:"queries"`
}

/*
Validate mysql config
*/
func (m *MySQLTaskConfig) Validate() (err error) {
	if len(m.Queries) == 0 {
		return fmt.Errorf("please provide at leas one query.")
	}

	for _, q := range m.Queries {
		if err = q.Validate(); err != nil {
			return
		}
	}

	return
}

/*
Configuration for single query
*/
type MySQLTaskConfigQuery struct {
	URL   string   `json:"url"`
	Query string   `json:"query"`
	Args  []string `json:"args"`
}

func (m *MySQLTaskConfigQuery) Validate() (err error) {
	m.URL = strings.TrimSpace(m.URL)
	if m.URL == "" {
		return fmt.Errorf("please provide url for query")
	}

	m.Query = strings.TrimSpace(m.Query)

	if m.Query == "" {
		return fmt.Errorf("please provide query")
	}
	return
}

/*
Factory to create task
*/
func MySQLTaskFactory(s *Server, tc *TaskConfig) (result []Tasker, err error) {
	config := &MySQLTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	result = []Tasker{&MySQLTask{
		config: config,
	}}
	return
}

/*
MySQL task imlpementation
*/
type MySQLTask struct {
	Task

	// configuration
	config *MySQLTaskConfig
}

/*
Run mysql task.
*/
func (m *MySQLTask) Run(r *http.Request, data map[string]interface{}) (res interface{}, status int, err error) {

	result := map[string]interface{}{}

	type Query struct {
		Query string                   `json:"query"`
		Rows  []map[string]interface{} `json:"rows,omitempty"`
		Args  []interface{}            `json:"args,omitempty"`
		Error string                   `json:"error,omitempty"`
		Code  int                      `json:"code,omitempty"`
	}

	queries := []Query{}

	var (
		db   *sqlx.DB
		rows *sqlx.Rows
	)

	for _, query := range m.config.Queries {

		args := []interface{}{}

		item := Query{
			Rows: []map[string]interface{}{},
		}

		var url string
		if url, err = m.Interpolate(query.URL, data); err != nil {
			item.Error = err.Error()
			goto Append
		}

		if m.config.ReturnQueries {
			item.Query = query.Query
		}

		if db, err = sqlx.Open("mysql", url); err != nil {
			if err, ok := err.(*mysql.MySQLError); ok {
				item.Error = err.Message
				item.Code = int(err.Number)
			} else {
				item.Error = err.Error()
			}

			goto Append
		}

		for _, arg := range query.Args {
			var a string

			if a, err = m.Interpolate(arg, data); err != nil {
				item.Error = err.Error()
				goto Append
			}

			args = append(args, a)
		}

		if m.config.ReturnQueries {
			item.Args = args
		}

		// run query
		rows, err = db.Queryx(item.Query, args...)
		if err != nil {
			if err, ok := err.(*mysql.MySQLError); ok {
				item.Error = err.Message
				item.Code = int(err.Number)
			} else {
				item.Error = err.Error()
			}
			goto Append
		}

		for rows.Next() {
			results := make(map[string]interface{})
			err = rows.MapScan(results)
			if err != nil {
				item.Error = err.Error()
				goto Append
			}
			item.Rows = append(item.Rows, results)
		}

	Append:
		queries = append(queries, item)
	}
	result["queries"] = queries

	res = result
	err = nil

	return
}
