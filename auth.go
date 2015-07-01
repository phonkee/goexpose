package goexpose

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"encoding/json"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

/*
Authorizer implements authorization
*/
type Authorizer interface {
	Authorize(r *http.Request) error
}

func init() {
	RegisterAuthorizer("basic", BasicAuthorizerFactory)
}

/*
AuthFactory returns new authorizer
*/
type AuthorizerFactory func(config *AuthorizerConfig) (Authorizer, error)

var (
	authorizers     = map[string]AuthorizerFactory{}
	authorizerslock = &sync.RWMutex{}
)

/*
Register authorizer
*/
func RegisterAuthorizer(id string, factory AuthorizerFactory) {
	authorizerslock.Lock()
	defer authorizerslock.Unlock()
	if _, ok := authorizers[id]; ok {
		panic(fmt.Sprintf("authorizer %s already registered", id))
	}
	authorizers[id] = factory
}

/*
AuthorizerExists returns if exists authorizer by given id
*/
func AuthorizerExists(id string) (ok bool) {
	authorizerslock.RLock()
	defer authorizerslock.RUnlock()
	_, ok = authorizers[id]
	return
}

/*
Returns authorizers for given config
First step is that it validates authorizers
*/
func GetAuthorizers(config *Config) (result Authorizers, err error) {
	result = Authorizers{}
	authorizerslock.RLock()
	defer authorizerslock.RUnlock()

	var authorizer Authorizer

	for an, ac := range config.Authorizers {

		_ = an
		// validate authorizer config
		if err = ac.Validate(); err != nil {
			return
		}

		var (
			factory AuthorizerFactory
			ok      bool
		)
		if factory, ok = authorizers[ac.Type]; !ok {
			err = fmt.Errorf("authorizer %s does not exist", ac.Type)
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
					err = fmt.Errorf("task %d, invalid authorizer `%s`.", i, a)
					return
				}
			}
		}
	}

	return
}

/*
Authorizers will have method that will check all authorizers
*/
type Authorizers map[string]Authorizer

/*
Try all authorizers, first that will fail with error, that error will be returned
*/
func (a Authorizers) Authorize(r *http.Request, config *EndpointConfig) (err error) {
	check := []string{}
	for _, an := range config.Authorizers {
		check = append(check, an)
	}
	for _, an := range config.Methods[r.Method].Authorizers {
		check = append(check, an)
	}

	for _, an := range check {
		authorizer := a[an]
		if err = authorizer.Authorize(r); err != nil {
			return
		}
	}
	return
}

/*
Returns names of all authorizerse
 */
func (a Authorizers) Names() []string {
	result := make([]string, 0, len(a))
	for k, _ := range a {
		result = append(result, k)
	}
	return result
}

/*
Basic auth provides method GetBasicAuth from request headers
*/
type BasicAuthorizer struct {
	config *BasicAuthorizerConfig
}

type BasicAuthorizerConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func BasicAuthorizerFactory(ac *AuthorizerConfig) (result Authorizer, err error) {
	config := &BasicAuthorizerConfig{}
	if err = json.Unmarshal(ac.Config, config); err != nil {
		return
	}

	result = &BasicAuthorizer{
		config: config,
	}
	return
}

var (
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header")
)

/*
Return username and password
*/
func (a *BasicAuthorizer) GetBasicAuth(r *http.Request) (username, password string, err error) {

	header := r.Header.Get("Authorization")
	splitted := strings.SplitN(header, " ", 2)
	if len(splitted) != 2 {
		err = ErrInvalidAuthorizationHeader
		return
	}

	if splitted[0] != "Basic" {
		err = ErrInvalidAuthorizationHeader
		return
	}
	var data []byte
	if data, err = base64.StdEncoding.DecodeString(splitted[1]); err != nil {
		err = ErrInvalidAuthorizationHeader
		return
	}

	up := strings.SplitN(string(data), ":", 2)
	if len(up) != 2 {
		err = ErrInvalidAuthorizationHeader
		return
	}

	username, password = up[0], up[1]

	return
}

/*
Check username and password
*/
func (b *BasicAuthorizer) Authorize(r *http.Request) (err error) {
	var username, password string

	if username, password, err = b.GetBasicAuth(r); err != nil {
		return
	}

	if username != b.config.Username || password != b.config.Password {
		return ErrUnauthorized
	}

	return
}
