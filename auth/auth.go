package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/config"
	"github.com/phonkee/goexpose/internal/utils"
	"net/http"
	"strings"
	"sync"

	"encoding/json"

	"net/url"

	"github.com/nmcclain/ldap"
)

// Authorizer implements authorization
type Authorizer interface {
	Authorize(r *http.Request) error
}

func init() {
	RegisterAuthorizer("basic", BasicAuthorizerInitFunc)
	RegisterAuthorizer("ldap", LDAPAuthorizerInitFunc)
	RegisterAuthorizer("http", HttpAuthorizerInitFunc)
}

// AuthorizerInitFunc returns new authorizer
type AuthorizerInitFunc func(config *config.AuthorizerConfig) (Authorizer, error)

var (
	authorizers     = map[string]AuthorizerInitFunc{}
	authorizerslock = &sync.RWMutex{}
)

func RegisterAuthorizer(id string, factory AuthorizerInitFunc) {
	authorizerslock.Lock()
	defer authorizerslock.Unlock()
	if _, ok := authorizers[id]; ok {
		panic(fmt.Sprintf("authorizer %s already registered", id))
	}
	authorizers[id] = factory
}

// AuthorizerExists returns if exists authorizer by given id
func AuthorizerExists(id string) (ok bool) {
	authorizerslock.RLock()
	defer authorizerslock.RUnlock()
	_, ok = authorizers[id]
	return
}

// GetAuthorizers Returns authorizers for given config
// First step is that it validates authorizers
func GetAuthorizers(config *config.Config) (result domain.Authorizers, err error) {
	result = domain.Authorizers{}
	authorizerslock.RLock()
	defer authorizerslock.RUnlock()

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

func BasicAuthorizerInitFunc(ac *config.AuthorizerConfig) (result Authorizer, err error) {
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
		return domain.ErrUnauthorized
	}

	return
}

/*
LDAP authorizer.

This authorizer handles username/password authentication from LDAP server.
It also supports blacklist/whitelist of users that are allowed to access goexpose endpoint.
*/

const (
	LDAP_DEFAULT_HOST    = "localhost"
	LDAP_DEFAULT_PORT    = 389
	LDAP_DEFAULT_NETWORK = "tcp"
)

type LDAPAuthorizerConfig struct {
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Network   string   `json:"network"`
	Whitelist []string `json:"whitelist"`
	Blacklist []string `json:"blacklist"`
}

/*
Validate configuration
*/
func (l *LDAPAuthorizerConfig) Validate() (err error) {
	if len(l.Whitelist) > 0 && len(l.Blacklist) > 0 {
		return domain.ErrBlacklistWhitelistProvided
	}

	found := false
	for _, an := range []string{"tcp", "tls"} {
		if an == l.Network {
			found = true
		}
	}

	if !found {
		return domain.ErrUnknownNetwork
	}

	return
}

func LDAPAuthorizerInitFunc(ac *config.AuthorizerConfig) (result Authorizer, err error) {
	config := &LDAPAuthorizerConfig{
		Host:    LDAP_DEFAULT_HOST,
		Port:    LDAP_DEFAULT_PORT,
		Network: LDAP_DEFAULT_NETWORK,
	}
	if err = json.Unmarshal(ac.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	result = &LDAPAuthorizer{
		config: config,
		basic:  &BasicAuthorizer{},
	}
	return
}

/*
LDAPAuthorizer
Main ldap authorizer implementation
*/
type LDAPAuthorizer struct {
	config *LDAPAuthorizerConfig

	basic *BasicAuthorizer
}

func (l *LDAPAuthorizer) Authorize(r *http.Request) (err error) {
	var (
		username string
		password string
	)
	if username, password, err = l.basic.GetBasicAuth(r); err != nil {
		return
	}

	var conn *ldap.Conn

	// dial correct network
	fullhost := fmt.Sprintf("%s:%d", l.config.Host, l.config.Port)
	if l.config.Network == "tcp" {
		conn, err = ldap.Dial("tcp", fullhost)
	} else if l.config.Network == "tls" {
		conn, err = ldap.DialTLS("tcp", fullhost, nil)
	}

	// check dial error
	if err != nil {
		return
	}

	// check blacklist
	if len(l.config.Blacklist) > 0 {
		for _, bl := range l.config.Blacklist {
			if bl == username {
				return domain.ErrBlacklisted
			}
		}
	}

	// check whitelist
	if len(l.config.Whitelist) > 0 {
		found := false
		for _, wl := range l.config.Whitelist {
			if wl == username {
				found = true
				break
			}
		}

		if !found {
			return domain.ErrNotWhitelisted
		}
	}

	// check ldap for username/password
	if err = conn.Bind(username, password); err != nil {
		return err
	}

	return
}

/*
http authorizer

http authorizer basically makes http request to check username and password against web service.
*/

func HttpAuthorizerInitFunc(ac *config.AuthorizerConfig) (result Authorizer, err error) {

	var (
		config *HttpAuthorizerConfig
	)

	// get config
	if config, err = NewHttpAuthorizerConfig(ac); err != nil {
		return
	}

	ha := &HttpAuthorizer{
		config: config,
	}

	result = ha
	return
}

/*
HttpAuthorizer implementation
*/
type HttpAuthorizer struct {
	config *HttpAuthorizerConfig
}

/*
Authorize main routine to allow user access
*/
func (h *HttpAuthorizer) Authorize(r *http.Request) (err error) {

	// prepare data for template interpolation
	data := map[string]interface{}{
		"username": "",
		"password": "",
	}

	var (
		url, method, body string
	)

	url, err = h.config.RenderURL(data)
	method, err = h.config.RenderMethod(data)
	body, err = h.config.RenderData(data)

	var (
		response *http.Response
	)

	// use Requester
	if _, response, err = utils.NewRequester().DoNew(method, url, strings.NewReader(body)); err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return domain.ErrUnauthorized
	}

	return
}

/*
NewHttpAuthorizerConfig

Returns fresh copy of AuthorizerConfi
*/

func NewHttpAuthorizerConfig(ac *config.AuthorizerConfig) (hac *HttpAuthorizerConfig, err error) {
	hac = &HttpAuthorizerConfig{
		Method: "GET",
		Data:   "",
	}

	if err = json.Unmarshal(ac.Config, hac); err != nil {
		return
	}

	var (
		parsed *url.URL
	)

	if parsed, err = url.Parse(hac.URL); err != nil {
		return
	}

	// update url
	hac.URL = parsed.String()

	// trim spaces
	hac.URL = strings.TrimSpace(hac.URL)
	hac.Data = strings.TrimSpace(hac.Data)
	hac.Method = strings.TrimSpace(hac.Method)

	return
}

/*
HttpAuthorizerConfig implementation

configuration for HttpAuthorizer
*/
type HttpAuthorizerConfig struct {
	URL    string `json:"url"`
	Data   string `json:"data"`
	Method string `json:"method"`
}

func (h *HttpAuthorizerConfig) RenderURL(data map[string]interface{}) (result string, err error) {
	return goexpose.RenderTextTemplate(h.URL, data)
}

func (h *HttpAuthorizerConfig) RenderData(data map[string]interface{}) (result string, err error) {
	return goexpose.RenderTextTemplate(h.Data, data)
}

func (h *HttpAuthorizerConfig) RenderMethod(data map[string]interface{}) (result string, err error) {
	return goexpose.RenderTextTemplate(h.Method, data)
}
