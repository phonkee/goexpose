package auth

import (
	"encoding/json"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/config"
	"github.com/phonkee/goexpose/internal/utils"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	RegisterAuthorizer("http", HttpAuthorizerInitFunc)
}

/*
http authorizer

http authorizer basically makes http request to check username and password against web service.
*/

func HttpAuthorizerInitFunc(ac *config.AuthorizerConfig) (result Authorizer, err error) {

	var (
		cfg *HttpAuthorizerConfig
	)

	// get cfg
	if cfg, err = NewHttpAuthorizerConfig(ac); err != nil {
		return
	}

	ha := &HttpAuthorizer{
		config: cfg,
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
		uurl, method, body string
	)

	uurl, err = h.config.RenderURL(data)
	method, err = h.config.RenderMethod(data)
	body, err = h.config.RenderData(data)

	var (
		response *http.Response
	)

	// use Requester
	if _, response, err = utils.NewRequester().DoNew(method, uurl, strings.NewReader(body)); err != nil {
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
		Method: http.MethodGet,
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
