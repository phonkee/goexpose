package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/tasks/registry"
	"net/http"
)

func init() {
	registry.RegisterTaskFactory("multi", MultiTaskFactory)
}

// MultiTaskFactory Factory to create task
func MultiTaskFactory(s domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (result []domain.Task, err error) {
	config := &MultiTaskConfig{
		Tasks: []*domain.TaskConfig{},
	}
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	mt := &MultiTask{
		config: config,
		tasks:  []domain.Task{},
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
			factory domain.TaskFactory
			ok      bool
			tasks   []domain.Task
		)

		if factory, ok = registry.GetTaskFactory(mtc.Type); !ok {
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

	result = []domain.Task{mt}
	return
}

type MultiTaskConfig struct {
	Tasks             []*domain.TaskConfig `json:"tasks"`
	SingleResult      *int                 `json:"single_result"`
	singleResultIndex int
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

// MultiTask imlpementation
type MultiTask struct {
	domain.BaseTask

	// configuration
	config *MultiTaskConfig
	tasks  []domain.Task
}

// Run multi task.
func (m *MultiTask) Run(r *http.Request, data map[string]interface{}) response.Response {
	results := make([]response.Response, 0)

	for _, tasker := range m.tasks {
		tr := tasker.Run(r, data)
		results = append(results, tr)
	}

	if m.config.singleResultIndex != -1 {
		return response.Result(results[m.config.singleResultIndex])
	}
	return response.Result(results)
}
