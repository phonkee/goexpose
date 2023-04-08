package tasks

import (
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/tasks/registry"
)

// Register additional tasks
func Register(name string, init domain.TaskInitFunc) {
	registry.RegisterTaskInitFunc(name, init)
}
