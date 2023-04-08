package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/tasks/registry"
	"net/http"
)

func init() {
	registry.RegisterTaskInitFunc("redis", RedisTaskFactory)
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
	singleResultIndex int
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

// Factory to create task instances
func RedisTaskFactory(server domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (result []domain.Task, err error) {
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

	result = []domain.Task{
		&RedisTask{
			config: config,
		},
	}
	return
}

type RedisTask struct {
	domain.BaseTask

	// config instance
	config *RedisTaskConfig
}

/*
Run method runs when request comes...
*/
func (r *RedisTask) Run(req *http.Request, data map[string]interface{}) response.Response {

	var (
		address string
		err     error
	)
	if address, err = goexpose.RenderTextTemplate(r.config.Address, data); err != nil {
		return response.Error(err)
	}

	var conn redis.Conn
	if conn, err = redis.Dial(r.config.Network, address); err != nil {
		return response.Error(err)
	}

	queries := make([]response.Response, 0)

	var (
		reply interface{}
		grr   interface{}
	)
	for _, query := range r.config.Queries {
		qr := response.OK()

		args := make([]interface{}, 0)
		for _, arg := range query.Args {
			var ia string
			if ia, err = goexpose.RenderTextTemplate(arg, data); err != nil {
				qr = qr.Error(err)
				goto AddItem
			}
			args = append(args, ia)
		}

		// return full query?
		if r.config.ReturnQueries {
			qr = qr.Data("command", query.Command)
			qr = qr.Data("args", args)
		}

		if reply, err = conn.Do(query.Command, args...); err != nil {
			qr = qr.Error(err)
			goto AddItem
		}

		// not found (not nice but..)
		if reply == nil {
			qr = qr.Error(errors.New("not found"))
			goto AddItem
		}

		if grr, err = r.GetReply(reply, query); err != nil {
			qr = qr.Error(err)
			goto AddItem
		}

		qr = qr.Result(grr)

	AddItem:
		queries = append(queries, qr)
	}

	// single result
	if r.config.singleResultIndex != -1 {
		response.Result(queries[r.config.singleResultIndex])
	}
	return response.Result(queries)
}

func (r *RedisTask) GetReply(reply interface{}, query RedisTaskConfigQuery) (interface{}, error) {
	if fn, ok := redisunderstands[query.Type]; !ok {
		return nil, fmt.Errorf("unknown redis type %s", query.Type)
	} else {
		return fn(reply, nil)
	}
}
