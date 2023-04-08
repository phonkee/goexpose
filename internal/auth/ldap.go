package auth

import (
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-defaults"
	"github.com/nmcclain/ldap"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/config"
	"net/http"
)

func init() {
	RegisterAuthorizer("ldap", LDAPAuthorizerInitFunc)
}

/*
LDAP authorizer.

This authorizer handles username/password authentication from LDAP server.
It also supports blacklist/whitelist of users that are allowed to access goexpose endpoint.
*/

type LDAPAuthorizerConfig struct {
	Host      string   `json:"host" default:"localhost"`
	Port      int      `json:"port" default:"389"`
	Network   string   `json:"network" default:"tcp"`
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
	cfg := &LDAPAuthorizerConfig{}
	defaults.SetDefaults(cfg)

	if err = json.Unmarshal(ac.Config, cfg); err != nil {
		return
	}

	if err = cfg.Validate(); err != nil {
		return
	}

	result = &LDAPAuthorizer{
		config: cfg,
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
