package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/tasks/registry"
	"net/http"
	"strings"
)

func init() {
	registry.RegisterTaskInitFunc("mysql", MySQLTaskInitFunc)
}

/*
MySQLTask

run queries on mysql
*/

type MySQLTaskConfig struct {
	ReturnQueries     bool                    `json:"return_queries"`
	Queries           []*MySQLTaskConfigQuery `json:"queries"`
	SingleResult      *int                    `json:"single_result"`
	singleResultIndex int
}

/*
Validate mysql config
*/
func (m *MySQLTaskConfig) Validate() (err error) {
	if len(m.Queries) == 0 {
		return fmt.Errorf("please provide at leas one query")
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

// MySQLTaskConfigQuery for single query
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

// MySQLTaskInitFunc to create task
func MySQLTaskInitFunc(s domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (result []domain.Task, err error) {
	config := &MySQLTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	result = []domain.Task{&MySQLTask{
		config: config,
	}}
	return
}

// MySQLTask task implementation
type MySQLTask struct {
	domain.BaseTask

	// configuration
	config *MySQLTaskConfig
}

// Run mysql task.
func (m *MySQLTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	queries := make([]response.Response, 0)

	var (
		db   *sqlx.DB
		rows *sqlx.Rows
		err  error
	)

	for _, query := range m.config.Queries {

		var (
			Rows []map[string]interface{}
		)

		args := make([]interface{}, 0)

		qr := response.OK()

		var url string
		if url, err = goexpose.RenderTextTemplate(query.URL, data); err != nil {
			qr = qr.Error(err)
			goto Append
		}

		if m.config.ReturnQueries {
			qr = qr.Data("query", query.Query)
		}

		if db, err = sqlx.Open("mysql", url); err != nil {
			qr = qr.Error(err.Error())
			if err, ok := err.(*mysql.MySQLError); ok {
				qr = qr.Data("error_code", err.Number)
			}
			goto Append
		}

		for _, arg := range query.Args {
			var a string

			if a, err = goexpose.RenderTextTemplate(arg, data); err != nil {
				qr = qr.Error(err)
				goto Append
			}

			args = append(args, a)
		}

		if m.config.ReturnQueries {
			qr = qr.Data("args", args)
		}

		// run query
		rows, err = db.Queryx(query.Query, args...)
		if err != nil {
			qr.Error(err)
			if err, ok := err.(*mysql.MySQLError); ok {
				qr = qr.Data("error_code", err.Number)
			}
			goto Append
		}

		Rows = []map[string]interface{}{}
		for rows.Next() {
			results := make(map[string]interface{})
			err = rows.MapScan(results)
			if err != nil {
				qr.Error(err)
				goto Append
			}
			Rows = append(Rows, results)
		}
		qr.Result(Rows)

	Append:
		queries = append(queries, qr)
	}

	// single result
	if m.config.singleResultIndex != -1 {
		return response.Result(queries[m.config.singleResultIndex])
	}
	return response.Result(queries)
}
