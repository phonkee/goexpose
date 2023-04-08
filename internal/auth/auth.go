package auth

import (
	"fmt"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/config"
	"net/http"
	"sync"
)

// Authorizer implements authorization
type Authorizer interface {
	Authorize(r *http.Request) error
}

// AuthorizerInitFunc returns new authorizer
type AuthorizerInitFunc func(config *config.AuthorizerConfig) (Authorizer, error)

var (
	authorizers     = map[string]AuthorizerInitFunc{}
	authorizersLock = &sync.RWMutex{}
)

func RegisterAuthorizer(id string, factory AuthorizerInitFunc) {
	authorizersLock.Lock()
	defer authorizersLock.Unlock()
	if _, ok := authorizers[id]; ok {
		panic(fmt.Sprintf("authorizer %s already registered", id))
	}
	authorizers[id] = factory
}

// AuthorizerExists returns if exists authorizer by given id
func AuthorizerExists(id string) (ok bool) {
	authorizersLock.RLock()
	defer authorizersLock.RUnlock()
	_, ok = authorizers[id]
	return
}

// GetAuthorizers Returns authorizers for given config
// First step is that it validates authorizers
func GetAuthorizers(config *config.Config) (result domain.Authorizers, err error) {
	result = domain.Authorizers{}
	authorizersLock.RLock()
	defer authorizersLock.RUnlock()

	var authorizer Authorizer

	for an, ac := range config.Authorizers {
		// validate authorizer config
		if err = ac.Validate(); err != nil {
			return
		}

		var (
			factory AuthorizerInitFunc
			ok      bool
		)
		if factory, ok = authorizers[ac.Type]; !ok {
			err = fmt.Errorf("authorizer %s does not exist", ac.Type)
			return
		}

		if authorizer, err = factory(ac); err != nil {
			return
		}
		result[an] = authorizer
	}

	// check task authorizers
	for i, ec := range config.Endpoints {
		for _, tc := range ec.Methods {
			for _, a := range tc.Authorizers {
				if _, ok := result[a]; !ok {
					err = fmt.Errorf("task %d, invalid authorizer `%s`", i, a)
					return
				}
			}
		}
	}

	return
}
