package registry

import (
	"fmt"
	"github.com/phonkee/goexpose/domain"
	"sync"
)

var (
	taskRegistry     = map[string]domain.TaskInitFunc{}
	taskRegistryLock = &sync.RWMutex{}
)

// RegisterTaskInitFunc registers task factory to server
func RegisterTaskInitFunc(id string, factory domain.TaskInitFunc) {
	taskRegistryLock.Lock()
	defer taskRegistryLock.Unlock()

	if _, ok := taskRegistry[id]; ok {
		panic(fmt.Sprintf("task factory %s already exists", id))
	}

	// add to registry
	taskRegistry[id] = factory
	return
}

// GetTaskInitFunc returns task factory by id
func GetTaskInitFunc(id string) (factory domain.TaskInitFunc, ok bool) {
	taskRegistryLock.RLock()
	defer taskRegistryLock.RUnlock()

	factory, ok = taskRegistry[id]
	return
}
