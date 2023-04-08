package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/tasks/registry"
	"github.com/phonkee/goexpose/internal/utils"
	"net/http"
	"strings"
)

func init() {
	registry.RegisterTaskInitFunc("cassandra", CassandraTaskInitFunc)
}

/*
Cassandra BaseTask

Run queries on cassandra cluster
*/

type CassandraTaskConfig struct {
	Queries           []CassandraTaskConfigQuery `json:"queries"`
	ReturnQueries     bool                       `json:"return_queries"`
	SingleResult      *int                       `json:"single_result"`
	singleResultIndex int
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
		return fmt.Errorf("please provide keyspace")
	}

	return
}

func CassandraTaskInitFunc(s domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (result []domain.Task, err error) {
	config := &CassandraTaskConfig{}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return

	}
	if err = config.Validate(); err != nil {
		return
	}
	result = []domain.Task{&CassandraTask{
		config: config,
	}}
	return
}

// Cassandra task to run queries on cassandra
type CassandraTask struct {
	domain.BaseTask

	// configuration
	config *CassandraTaskConfig
}

/*
Run cassandra task
*/
func (c *CassandraTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	queries := make([]response.Response, 0)

	for _, query := range c.config.Queries {
		var args []interface{}

		var (
			Result  []map[string]interface{}
			cluster *gocql.ClusterConfig
			session *gocql.Session
			err     error
		)

		qr := response.OK()

		var cHosts []string
		for _, i := range query.Cluster {
			var cHost string
			if cHost, err = utils.RenderTextTemplate(i, data); err != nil {
				qr.Error(err)
				goto Append
			}
			cHosts = append(cHosts, cHost)
		}

		// instantiate cluster
		cluster = gocql.NewCluster(cHosts...)
		if cluster.Keyspace, err = utils.RenderTextTemplate(query.Keyspace, data); err != nil {
			qr = qr.Error(err)
			goto Append
		}

		if session, err = cluster.CreateSession(); err != nil {
			qr = qr.Error(err)
			goto Append
		}

		if c.config.ReturnQueries {
			qr = qr.Data("query", query.Query)
		}

		for _, arg := range query.Args {
			final, err := utils.RenderTextTemplate(arg, data)
			if err != nil {
				qr = qr.Error(err)
				goto Append
			} else {
				args = append(args, final)
			}
		}

		if c.config.ReturnQueries {
			qr = qr.Data("args", args)
		}

		// slicemap to result
		if Result, err = session.Query(query.Query, args...).Iter().SliceMap(); err != nil {
			qr.Error(err)
			goto Append
		} else {
			qr.Result(Result)
			goto Append
		}

	Append:
		queries = append(queries, qr)
	}

	// single result
	if c.config.singleResultIndex != -1 {
		return response.Result(queries[c.config.singleResultIndex])
	}

	return response.Result(queries)
}
