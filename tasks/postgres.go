package tasks

import (
	"encoding/json"
	"errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"net/http"
)

func init() {
	goexpose.RegisterTaskFactory("postgres", PostgresTaskFactory)
}

/*
PostgresTask

run queries on postgres database
*/

func PostgresTaskFactory(server domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {
	config := &PostgresTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}
	if err = config.Validate(); err != nil {
		return
	}
	tasks = []domain.Task{&PostgresTask{
		config: config,
	}}

	return
}

type PostgresTaskConfig struct {
	Queries           []*PostgresTaskConfigQuery `json:"queries"`
	ReturnQueries     bool                       `json:"return_queries"`
	SingleResult      *int                       `json:"single_result"`
	singleResultIndex int
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

// Postgres task
type PostgresTask struct {
	domain.BaseTask

	// configuration
	config *PostgresTaskConfig
}

/*
Run postgres task
*/
func (p *PostgresTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	var queryResults []response.Response

	for _, query := range p.config.Queries {

		qresponse := response.OK()

		var (
			args []interface{}
			db   *sqlx.DB
			err  error
			url  string
			rows *sqlx.Rows
			Rows []map[string]interface{}

			errq error
		)
		if url, err = goexpose.Interpolate(query.URL, data); err != nil {
			qresponse = qresponse.Error(err)
			goto Append
		}

		// interpolate all args
		args = []interface{}{}
		for _, arg := range query.Args {
			interpolated, e := goexpose.Interpolate(arg, data)
			if e != nil {
				qresponse = qresponse.Error(e)
				goto Append
			}
			args = append(args, interpolated)
		}

		// add query with args to response?
		if p.config.ReturnQueries {
			qresponse = qresponse.Data("query", query.Query).Data("args", args)
		}

		if db, err = sqlx.Connect("postgres", url); err != nil {

			if err, ok := err.(*pq.Error); ok {
				qresponse = qresponse.Data("error_code", err.Code.Name())
			}
			qresponse.Error(err)
			goto Append
		}

		// run query
		rows, errq = db.Queryx(query.Query, args...)
		if errq != nil {
			if errq, ok := errq.(*pq.Error); ok {
				qresponse = qresponse.Data("error_code", errq.Code.Name())
			}
			qresponse.Error(errq)
			goto Append
		}

		Rows = []map[string]interface{}{}

		for rows.Next() {
			results := make(map[string]interface{})
			err = rows.MapScan(results)
			if err != nil {
				if err, ok := err.(*pq.Error); ok {
					qresponse = qresponse.Data("error_code", err.Code.Name())
				}
				qresponse.Error(err)
				goto Append
			}

			Rows = append(Rows, results)
		}
		qresponse.Result(Rows)

	Append:
		queryResults = append(queryResults, qresponse)
	}

	// single result
	if p.config.singleResultIndex != -1 {
		response.Result(queryResults[p.config.singleResultIndex])
	}
	return response.Result(queryResults)
}
