package goexpose

import (
	"fmt"
	"github.com/phonkee/goexpose/domain"
	"sync"
)

var (
	taskRegistry     = map[string]domain.TaskFactory{}
	taskRegistryLock = &sync.RWMutex{}
)

// RegisterTaskFactory registers task factory to server
func RegisterTaskFactory(id string, factory domain.TaskFactory) {
	taskRegistryLock.Lock()
	defer taskRegistryLock.Unlock()

	if _, ok := taskRegistry[id]; ok {
		panic(fmt.Sprintf("task factory %s already exists", id))
	}

	// add to registry
	taskRegistry[id] = factory
	return
}

// GetTaskFactory returns task factory by id
func GetTaskFactory(id string) (factory domain.TaskFactory, ok bool) {
	taskRegistryLock.RLock()
	defer taskRegistryLock.RUnlock()

	factory, ok = taskRegistry[id]
	return
}
