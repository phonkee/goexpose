package goexpose

import (
	"encoding/json"
	"net/http"

	"fmt"
	"os/exec"

	"io"
	"io/ioutil"
	"strings"

	"os"
	"path/filepath"

	"encoding/base64"
	"net/url"

	"github.com/garyburd/redigo/redis"
	"github.com/go-sql-driver/mysql"
	"github.com/gocql/gocql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/phonkee/wheedle/errors"
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
	RegisterTaskFactory("multi", MultiTaskFactory)
	RegisterTaskFactory("filesystem", FilesystemFactory)
}

/*
Config for info task
*/
type ShellTaskConfig struct {
	// Custom environment variables
	Env               map[string]string         `json:"env"`
	Shell             string                    `json:"shell"`
	Commands          []*ShellTaskConfigCommand `json:"commands"`
	SingleResult      *int                      `json:"single_result"`
	singleResultIndex int                       `json:"-"`
}

func (s *ShellTaskConfig) Validate() (err error) {
	if len(s.Commands) == 0 {
		return errors.New("please provide at least one command")
	}
	for _, c := range s.Commands {
		if err = c.Validate(); err != nil {
			return
		}
	}
	if s.SingleResult != nil {
		s.singleResultIndex = *s.SingleResult
		if s.singleResultIndex > len(s.Commands)-1 {
			return errors.New("single_result out of bounds")
		}
	} else {
		s.singleResultIndex = -1
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
func ShellTaskFactory(server *Server, taskconfig *TaskConfig, ec *EndpointConfig) (tasks []Tasker, err error) {
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
func (s *ShellTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	results := []*Response{}

	response = NewResponse(http.StatusOK)

	// run all commands
	for _, command := range s.Config.Commands {

		cmdresp := NewResponse(http.StatusOK)

		var (
			b            string
			e            error
			finalCommand string
			cmd          *exec.Cmd
		)
		if b, e = s.Interpolate(command.Command, data); e != nil {
			cmdresp.Status(http.StatusInternalServerError).Error(e)
			goto Append
		}

		finalCommand = b

		// show command in result
		if command.ReturnCommand {
			cmdresp.AddValue("command", finalCommand)
		}

		// run command
		cmd = exec.Command(s.Config.Shell, "-c", finalCommand)

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
			cmdresp.Status(http.StatusInternalServerError).Error(err)
			goto Append
		} else {
			// format out
			if re, f, e := Format(string(strings.TrimSpace(string(out))), command.Format); e == nil {
				cmdresp.Result(re).AddValue("format", f)
			} else {
				cmdresp.Status(http.StatusInternalServerError).Error(e)
			}
			goto Append
		}

	Append:
		results = append(results, cmdresp)
	}

	// single result
	if s.Config.singleResultIndex != -1 {
		response.Result(results[s.Config.singleResultIndex])
	} else {
		response.Result(results)
	}

	return
}

/*
Factory for InfoTask task
*/
func InfoTaskFactory(server *Server, taskconfig *TaskConfig, ec *EndpointConfig) (tasks []Tasker, err error) {

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
func (i *InfoTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {
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

	return NewResponse(http.StatusOK).Result(data)
}

/*
HttpTask configuration

Attrs:
Method - if blank, method from request will be used
Format - "json", "text", ""
	if blank json will be guessed from Content-Type header
*/

type HttpTaskConfig struct {
	SingleResult      *int                 `json:"single_result"`
	singleResultIndex int                  `json:"-"`
	URLs              []*HttpTaskConfigURL `json:"urls"`
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

	if len(h.URLs) == 0 {
		return fmt.Errorf("http task must provide at least one url")
	}
	for _, url := range h.URLs {
		url.URL = strings.TrimSpace(url.URL)
		if url.URL == "" {
			return fmt.Errorf("Invalid url in http task.")
		}

		if url.Format, err = VerifyFormat(url.Format); err != nil {
			return err
		}
	}

	if h.SingleResult != nil {
		h.singleResultIndex = *h.SingleResult
		if h.singleResultIndex > len(h.URLs)-1 {
			return errors.New("http task single_result out of bounds")
		}
	} else {
		h.singleResultIndex = -1
	}

	return
}

/*
HttpTaskFactory - factory to create HttpTasks
*/
func HttpTaskFactory(server *Server, tc *TaskConfig, ec *EndpointConfig) (tasks []Tasker, err error) {
	// default config
	config := &HttpTaskConfig{}

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
func (h *HttpTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	results := []*Response{}

	response = NewResponse(http.StatusOK)

	var err error

	for _, url := range h.config.URLs {

		ir := NewResponse(http.StatusOK)

		client := &http.Client{}
		var req *http.Request

		var body io.Reader

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
			ir.Status(http.StatusInternalServerError).Error(err)
			results = append(results, ir)
			continue
		}

		if req, err = http.NewRequest(method, b, body); err != nil {
			ir.Status(http.StatusInternalServerError).Error(err)
			results = append(results, ir)
			continue
		}

		var resp *http.Response
		if resp, err = client.Do(req); err != nil {
			ir.Status(http.StatusInternalServerError).Error(err)
			results = append(results, ir)
			continue
		}

		var respbody []byte
		if respbody, err = ioutil.ReadAll(resp.Body); err != nil {
			ir.Status(http.StatusInternalServerError).Error(err)
			results = append(results, ir)
			continue
		}

		// prepare response
		ir.Status(resp.StatusCode)

		// return headers?
		if url.ReturnHeaders {
			ir.AddValue("headers", resp.Header)
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
			ir.Result(re).AddValue("format", f)
		} else {
			ir.Error(e)
		}

		results = append(results, ir)
	}

	// return single result
	if h.config.singleResultIndex != -1 {
		response = results[h.config.singleResultIndex]
	} else {
		response.Result(results)
	}

	return
}

/*
PostgresTask

run queries on postgres database
*/

func PostgresTaskFactory(server *Server, tc *TaskConfig, ec *EndpointConfig) (tasks []Tasker, err error) {
	config := &PostgresTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}
	if err = config.Validate(); err != nil {
		return
	}
	tasks = []Tasker{&PostgresTask{
		config: config,
	}}

	return
}

type PostgresTaskConfig struct {
	Queries           []*PostgresTaskConfigQuery `json:"queries"`
	ReturnQueries     bool                       `json:"return_queries"`
	SingleResult      *int                       `json:"single_result"`
	singleResultIndex int                        `json:"-"`
}

func (p *PostgresTaskConfig) Validate() (err error) {
	if len(p.Queries) == 0 {
		return errors.New("please provide at least one queryS")
	}
	if p.SingleResult != nil {
		p.singleResultIndex = *p.SingleResult
		if p.singleResultIndex > len(p.Queries)-1 {
			return errors.New("postgres task single_result out of bounds")
		}
	} else {
		p.singleResultIndex = -1
	}
	return
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
func (p *PostgresTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	response = NewResponse(http.StatusOK)

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

	// single result
	if p.config.singleResultIndex != -1 {
		response.Result(queryresults[p.config.singleResultIndex])
	} else {
		response.Result(queryresults)
	}

	return
}

/*
RedisTask

task to call commands on redis
*/

type RedisTaskConfig struct {
	Address           string                 `json:"address"`
	Database          int                    `json:"database"`
	Network           string                 `json:"network"`
	Queries           []RedisTaskConfigQuery `json:"queries"`
	ReturnQueries     bool                   `json:"return_queries"`
	SingleResult      *int                   `json:"single_result"`
	singleResultIndex int                    `json:"-"`
}

func (r *RedisTaskConfig) Validate() (err error) {

	if len(r.Queries) == 0 {
		return errors.New("please provide at least one query.")
	}
	for _, i := range r.Queries {
		if err = i.Validate(); err != nil {
			return
		}
	}

	if r.SingleResult != nil {
		r.singleResultIndex = *r.SingleResult
		if r.singleResultIndex > len(r.Queries)-1 {
			return errors.New("single_result out of bounds")
		}
	} else {
		r.singleResultIndex = -1
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
func RedisTaskFactory(server *Server, tc *TaskConfig, ec *EndpointConfig) (result []Tasker, err error) {
	// address defaults to tcp
	config := &RedisTaskConfig{
		Address:  ":6379",
		Network:  "tcp",
		Database: 1,
	}

	// unmarshall config
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	// validate config
	if err = config.Validate(); err != nil {
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
func (rt *RedisTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	response = NewResponse(http.StatusOK)

	type Item struct {
		Error   string        `json:"error,omitempty"`
		Result  interface{}   `json:"result,omitempty"`
		Command string        `json:"command,omitempty"`
		Args    []interface{} `json:"args,omitempty"`
	}

	var (
		address string
		err     error
	)
	if address, err = rt.Interpolate(rt.config.Address, data); err != nil {
		response.Error(err)
		return
	}

	var conn redis.Conn
	if conn, err = redis.Dial(rt.config.Network, address); err != nil {
		response.Error(err)
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

	// single result
	if rt.config.singleResultIndex != -1 {
		response.Result(queries[rt.config.singleResultIndex])
	} else {
		response.Result(queries)
	}

	return
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
	Queries           []CassandraTaskConfigQuery `json:"queries"`
	ReturnQueries     bool                       `json:"return_queries"`
	SingleResult      *int                       `json:"single_result"`
	singleResultIndex int                        `json:"-"`
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

	if c.SingleResult != nil {
		c.singleResultIndex = *c.SingleResult
		if c.singleResultIndex > len(c.Queries)-1 {
			return errors.New("single_result out of bounds")
		}
	} else {
		c.singleResultIndex = -1
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

func CassandraTaskFactory(s *Server, tc *TaskConfig, ec *EndpointConfig) (result []Tasker, err error) {
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
func (c *CassandraTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	response = NewResponse(http.StatusOK)

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

	var err error

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

	// single result
	if c.config.singleResultIndex != -1 {
		response.Result(result.Queries[c.config.singleResultIndex])
	} else {
		response.Result(result)
	}

	return
}

/*
MySQLTask

run queries on mysql
*/

type MySQLTaskConfig struct {
	ReturnQueries     bool                    `json:"return_queries"`
	Queries           []*MySQLTaskConfigQuery `json:"queries"`
	SingleResult      *int                    `json:"single_result"`
	singleResultIndex int                     `json:"-"`
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

	if m.SingleResult != nil {
		m.singleResultIndex = *m.SingleResult
		if m.singleResultIndex > len(m.Queries)-1 {
			return errors.New("single_result out of bounds")
		}
	} else {
		m.singleResultIndex = -1
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
func MySQLTaskFactory(s *Server, tc *TaskConfig, ec *EndpointConfig) (result []Tasker, err error) {
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
func (m *MySQLTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	response = NewResponse(http.StatusOK)

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
		err  error
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

	// single result
	if m.config.singleResultIndex != -1 {
		response.Result(queries[m.config.singleResultIndex])
	} else {
		response.Result(queries)
	}

	return
}

/*
Factory to create task
*/
func MultiTaskFactory(s *Server, tc *TaskConfig, ec *EndpointConfig) (result []Tasker, err error) {
	config := &MultiTaskConfig{
		Tasks: []*TaskConfig{},
	}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	mt := &MultiTask{
		config: config,
		tasks:  []Tasker{},
	}

	for _, mtc := range config.Tasks {

		if mtc.Type == "multi" {
			err = errors.New("multi task does not support embedded multi tasks")
			return
		}

		// validate task config
		if err = mtc.Validate(); err != nil {
			return
		}

		var (
			factory TaskFactory
			ok      bool
			tasks   []Tasker
		)

		if factory, ok = getTaskFactory(mtc.Type); !ok {
			err = fmt.Errorf("task %s doesn't exist", mtc.Type)
			return
		}

		if tasks, err = factory(s, mtc, ec); err != nil {
			err = fmt.Errorf("task %s returned error: %s", mtc.Type, err)
			return
		}

		// append all tasks
		for _, t := range tasks {
			mt.tasks = append(mt.tasks, t)
		}
	}

	result = []Tasker{mt}
	return
}

type MultiTaskConfig struct {
	Tasks             []*TaskConfig `json:"tasks"`
	SingleResult      *int          `json:"single_result"`
	singleResultIndex int           `json:"-"`
}

func (m *MultiTaskConfig) Validate() (err error) {
	if len(m.Tasks) == 0 {
		return errors.New("multi task must have at least one task")
	}

	if m.SingleResult != nil {
		m.singleResultIndex = *m.SingleResult
		if m.singleResultIndex > len(m.Tasks)-1 {
			return errors.New("multi task single_result out of bounds")
		}
	} else {
		m.singleResultIndex = -1
	}
	return
}

/*
Multi task imlpementation
*/
type MultiTask struct {
	Task

	// configuration
	config *MultiTaskConfig
	tasks  []Tasker
}

/*
Run multi task.
*/
func (m *MultiTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	response = NewResponse(http.StatusOK)

	results := []*Response{}

	for _, tasker := range m.tasks {
		tr := tasker.Run(r, data)
		results = append(results, tr)
	}

	if m.config.singleResultIndex != -1 {
		response.Result(results[m.config.singleResultIndex])
	} else {
		response.Result(results)
	}

	return
}

/*
Filesystem task gives possibility to serve files. It operates in two modes: file, directory.
	file:
		serves single file
	directory:
		serves all files in folder
*/

func NewFilesystemConfig() *FilesystemConfig {
	return &FilesystemConfig{
		Mode:       "file",
		FileMuxVar: "file",
	}
}

type FilesystemConfig struct {
	Mode       string `json:"mode"`
	File       string `json:"file"`
	Directory  string `json:"directory"`
	Index      bool   `json:"index"`
	FileMuxVar string `json:"file_url_var"`
}

/*
@TODO: add additional checks (file/directory existency)
*/
func (f *FilesystemConfig) Validate() (err error) {
	// cleanup strings
	f.Mode = strings.TrimSpace(f.Mode)
	f.File = strings.TrimSpace(f.File)
	f.Directory = strings.TrimSpace(f.Directory)
	f.FileMuxVar = strings.TrimSpace(f.FileMuxVar)

	// perform validations
	switch f.Mode {
	case "file":
		if f.File == "" {
			return errors.New("please provide file")
		}
		if f.Directory != "" {
			return errors.New("directory set for mode file")
		}
	case "directory":
		if f.File != "" {
			return errors.New("file set for mode directory")
		}
		if f.Directory == "" {
			return errors.New("please provide directory")
		}
		if f.FileMuxVar == "" {
			return errors.New("please provide valid file_url_var")
		}

		// get absolute path
		if f.Directory, err = filepath.Abs(f.Directory); err != nil {
			return fmt.Errorf("directory abs returned error %s", err)
		}

		// check directory
		if _, err = ioutil.ReadDir(f.Directory); err != nil {
			return fmt.Errorf("directory error: %s", err)
		}
	default:
		return fmt.Errorf("unknown filesystem mode: %s", f.Mode)
	}
	return
}

/*
FilesystemFileTask
	serve single file
*/
type FilesystemFileTask struct {
	Task
	config   *FilesystemConfig
	endpoint *EndpointConfig
	server   *Server
}

func (f *FilesystemFileTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {
	return NewResponse(http.StatusOK)
}

/*
FilesystemDirectoryTask
	serve single file
*/
type FilesystemDirectoryTask struct {
	Task
	config   *FilesystemConfig
	endpoint *EndpointConfig
	server   *Server
}

func (f *FilesystemDirectoryTask) Run(r *http.Request, data map[string]interface{}) (response *Response) {

	var (
		muxvar string
		ok     bool
	)

	response = NewResponse(http.StatusOK)

	urldata := data["url"].(map[string]string)

	if muxvar, ok = urldata[f.config.FileMuxVar]; !ok {
		return response.Status(http.StatusInternalServerError).Error(fmt.Errorf("file_url_var %s not found", f.config.FileMuxVar))
	}

	final := filepath.Join(f.config.Directory, muxvar)
	_ = final

	var (
		err error
		fi  os.FileInfo
	)
	if fi, err = os.Stat(final); err != nil {
		return response.Status(http.StatusNotFound)
	}

	if fi.IsDir() {
		if !f.config.Index {
			return response.Status(http.StatusNotFound)
		}

		var items []os.FileInfo
		if items, err = ioutil.ReadDir(final); err != nil {
			return response.Status(http.StatusInternalServerError)
		}

		result := []map[string]interface{}{}

		for _, item := range items {

			var u *url.URL
			if u, err = f.server.Router.Get(f.endpoint.RouteName()).URL(f.config.FileMuxVar, filepath.Join(muxvar, item.Name())); err != nil {
				return response.Status(http.StatusInternalServerError)
			}

			ri := map[string]interface{}{
				"name":   u.Path,
				"is_dir": item.IsDir(),
			}
			result = append(result, ri)
		}

		return response.Result(result)
	}

	// is file so serve it please

	var contents []byte
	if contents, err = ioutil.ReadFile(final); err != nil {
		return response.Status(http.StatusInternalServerError)
	}

	//	mode := r.URL.Query().Get("mode")
	//	if mode == "raw" {
	//	}

	b64content := base64.StdEncoding.EncodeToString(contents)
	_, ff := filepath.Split(final)

	return response.Result(map[string]string{
		"name":     ff,
		"contents": b64content,
	})
}

func FilesystemFactory(s *Server, tc *TaskConfig, ec *EndpointConfig) (result []Tasker, err error) {
	var task Tasker

	config := NewFilesystemConfig()
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	switch config.Mode {
	case "file":
		task = &FilesystemFileTask{
			config:   config,
			endpoint: ec,
			server:   s,
		}
	case "directory":
		task = &FilesystemDirectoryTask{
			config:   config,
			endpoint: ec,
			server:   s,
		}
	}

	return []Tasker{task}, err
}
