package goexpose

import (
	"fmt"
	"sync"
)

var (
	taskregistry     = map[string]TaskFactory{}
	taskregistrylock = &sync.RWMutex{}
)

/*
Registers task factory to server
*/
func RegisterTaskFactory(id string, factory TaskFactory) {
	taskregistrylock.Lock()
	defer taskregistrylock.Unlock()

	if _, ok := taskregistry[id]; ok {
		panic(fmt.Sprintf("task factory %s already exists", id))
	}

	// add to registry
	taskregistry[id] = factory
	return
}

/*
Returns task factory by id
*/
func getTaskFactory(id string) (factory TaskFactory, ok bool) {
	taskregistrylock.RLock()
	defer taskregistrylock.RUnlock()

	factory, ok = taskregistry[id]
	return
}
