package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/phonkee/goexpose/domain"
	"net/http"
	"strings"
)

func init() {
	RegisterAuthorizer("basic", BasicAuthorizerInitFunc)
}

// TODO: we should have ability to fetch usernames/passwords from file
type BasicAuthorizer struct {
	config *BasicAuthorizerConfig
}

type BasicAuthorizerConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func BasicAuthorizerInitFunc(ac *domain.AuthorizerConfig) (result Authorizer, err error) {
	cfg := &BasicAuthorizerConfig{}
	if err = json.Unmarshal(ac.Config, cfg); err != nil {
		return
	}

	result = &BasicAuthorizer{
		config: cfg,
	}
	return
}

var (
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header")
)

// GetBasicAuth Return username and password
func (b *BasicAuthorizer) GetBasicAuth(r *http.Request) (username, password string, err error) {

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

// Authorize Checks username and password
func (b *BasicAuthorizer) Authorize(r *http.Request) (err error) {
	var username, password string

	if username, password, err = b.GetBasicAuth(r); err != nil {
		return
	}

	if username != b.config.Username || password != b.config.Password {
		return domain.ErrUnauthorized
	}

	return
}
