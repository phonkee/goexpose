package goexpose

import (
	"fmt"
	"github.com/phonkee/goexpose/domain"
	"sync"
)

var (
	taskregistry     = map[string]func(server Server, config *domain.TaskConfig, ec *domain.EndpointConfig) ([]domain.Tasker, error){}
	taskregistrylock = &sync.RWMutex{}
)

/*
Registers task factory to server
*/
func RegisterTaskFactory(id string, factory func(server Server, config *domain.TaskConfig, ec *domain.EndpointConfig) ([]func(server Server, config *domain.TaskConfig, ec *domain.EndpointConfig) ([]domain.Tasker, error), error)) {
	taskregistrylock.Lock()
	defer taskregistrylock.Unlock()

	if _, ok := taskregistry[id]; ok {
		panic(fmt.Sprintf("task factory %s already exists", id))
	}

	// add to registry
	taskregistry[id] = factory
	return
}

// GetTaskFactory returns task factory by id
func GetTaskFactory(id string) (factory func(server Server, config *domain.TaskConfig, ec *domain.EndpointConfig) ([]func(server Server, config *domain.TaskConfig, ec *domain.EndpointConfig) ([]domain.Tasker, error), error), ok bool) {
	taskregistrylock.RLock()
	defer taskregistrylock.RUnlock()

	factory, ok = taskregistry[id]
	return
}
