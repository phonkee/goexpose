package response

import (
	"errors"
	"net/http"
	"sync"

	gerrors "github.com/pkg/errors"
)

var (
	em *errMap
)

func init() {
	// initialize global error map
	em = newErrMap()
}

func newErrMap() *errMap {
	return &errMap{
		mutex:    &sync.RWMutex{},
		registry: map[error]int{},
	}
}

// RegisterError registers error to given http status
func RegisterError(err error, status int) {
	em.Register(err, status)
}

// GetErrorStatus returns appropriate http status for given error
func GetErrorStatus(err error) int {
	return em.GetStatus(err)
}

// errMap provides mapping from errors to http statuses
// then you can call response.Error(myerr) it will set appropriate status
type errMap struct {

	// mutex to secure our map
	mutex *sync.RWMutex

	// store map from error to int
	registry map[error]int
}

// Register registers error
func (e *errMap) Register(err error, status int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.registry[err] = status
	return
}

// GetStatus returns appropriate status for given error
func (e *errMap) GetStatus(err error) (status int) {
	var ok bool

	defer func() {
		if r := recover(); r != nil {
			status = http.StatusInternalServerError
		}
	}()

	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// try multiple sources
	for _, errFound := range []error{err, gerrors.Cause(err)} {
		if errFound == nil {
			continue
		}
		if status, ok = e.registry[errFound]; ok {
			return
		}
	}

	for {
		if err = errors.Unwrap(err); err == nil {
			break
		}
		if status, ok = e.registry[err]; ok {
			return
		}
	}

	return 0
}
