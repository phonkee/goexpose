package goexpose

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"encoding/json"

	"net/url"

	"github.com/nmcclain/ldap"
)

var (
	ErrUnauthorized               = errors.New("unauthorized")
	ErrBlacklisted                = errors.New("user is blacklisted")
	ErrNotWhitelisted             = errors.New("user is not whitelisted")
	ErrBlacklistWhitelistProvided = errors.New("blacklist and whitelist set, that doesn't make sense.")
	ErrUnknownNetwork             = errors.New("unknown network")
	ErrURLInvalidTemplate         = errors.New("url is invalid template")
)

/*
Authorizer implements authorization
*/
type Authorizer interface {
	Authorize(r *http.Request) error
}

func init() {
	RegisterAuthorizer("basic", BasicAuthorizerFactory)
	RegisterAuthorizer("ldap", LDAPAuthorizerFactory)
	RegisterAuthorizer("http", HttpAuthorizerFactory)
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
		return ErrBlacklistWhitelistProvided
	}

	found := false
	for _, an := range []string{"tcp", "tls"} {
		if an == l.Network {
			found = true
		}
	}

	if !found {
		return ErrUnknownNetwork
	}

	return
}

func LDAPAuthorizerFactory(ac *AuthorizerConfig) (result Authorizer, err error) {
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
				return ErrBlacklisted
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
			return ErrNotWhitelisted
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

func HttpAuthorizerFactory(ac *AuthorizerConfig) (result Authorizer, err error) {

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
		"username": "phonkee",
		"password": "password",
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
	if _, response, err = NewRequester().DoNew(method, url, strings.NewReader(body)); err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return ErrUnauthorized
	}

	return
}

/*
NewHttpAuthorizerConfig

Returns fresh copy of AuthorizerConfi
*/

func NewHttpAuthorizerConfig(ac *AuthorizerConfig) (hac *HttpAuthorizerConfig, err error) {
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
	return Interpolate(h.URL, data)
}

func (h *HttpAuthorizerConfig) RenderData(data map[string]interface{}) (result string, err error) {
	return Interpolate(h.Data, data)
}

func (h *HttpAuthorizerConfig) RenderMethod(data map[string]interface{}) (result string, err error) {
	return Interpolate(h.Method, data)
}
