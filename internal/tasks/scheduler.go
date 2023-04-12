package tasks

import (
	"encoding/json"
	"github.com/mcuadros/go-defaults"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/tasks/registry"
	"net/http"
)

func init() {
	registry.RegisterTaskInitFunc("scheduler", SchedulerTaskFactory)
}

// SchedulerTaskFactory creates scheduler task from single task config
func SchedulerTaskFactory(server domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) ([]domain.Task, error) {
	cfg := &SchedulerConfig{}

	// set default values
	defaults.SetDefaults(&cfg)

	if err := json.Unmarshal(tc.Config, cfg); err != nil {
		return nil, err
	}

	return []domain.Task{&SchedulerTask{
		config: cfg,
	}}, nil
}

type SchedulerConfig struct {
	Cron string `json:"cron,omitempty"`
}

type SchedulerTask struct {
	domain.BaseTask
	config *SchedulerConfig
}

func (s *SchedulerTask) Run(r *http.Request, vars map[string]interface{}) response.Response {
	//TODO implement me
	panic("implement me")
}
