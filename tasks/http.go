package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func init() {
	goexpose.RegisterTaskFactory("http", HttpTaskFactory)
}

/*
HttpTask configuration

Attrs:
Method - if blank, method from request will be used
Format - "json", "text", ""
	if blank json will be guessed from Content-Type header
*/

type HttpTaskConfig struct {
	URLs         []*HttpTaskConfigURL `json:"urls"`
	SingleResult *int                 `json:"single_result"`

	// computed property
	singleResultIndex int `json:"-"`
}

type HttpTaskConfigURL struct {
	URL           string `json:"url"`
	Method        string `json:"method"`
	PostBody      bool   `json:"post_body"`
	Format        string `json:"format"`
	ReturnHeaders bool   `json:"return_headers"`
}

/*
Validate config
*/
func (h *HttpTaskConfig) Validate() (err error) {

	if len(h.URLs) == 0 {
		return fmt.Errorf("http task must provide at least one url")
	}
	for _, url := range h.URLs {
		url.URL = strings.TrimSpace(url.URL)
		if url.URL == "" {
			return fmt.Errorf("Invalid url in http task.")
		}

		if url.Format, err = goexpose.VerifyFormat(url.Format); err != nil {
			return err
		}
	}

	if h.SingleResult != nil {
		h.singleResultIndex = *h.SingleResult
		if h.singleResultIndex > len(h.URLs)-1 {
			return errors.New("http task single_result out of bounds")
		}
	} else {
		h.singleResultIndex = -1
	}

	return
}

/*
HttpTaskFactory - factory to create HttpTasks
*/
func HttpTaskFactory(server domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {
	// default config
	config := &HttpTaskConfig{}

	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	// return tasks
	tasks = []domain.Task{&HttpTask{
		config: config,
	}}
	return
}

/*
HttpTask

	task that can make requests to given urls
*/
type HttpTask struct {
	domain.BaseTask

	// http configuration
	config *HttpTaskConfig
}

/*
Run method is called on request
@TODO: please refactor me!
*/
func (h *HttpTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	results := make([]*goexpose.Response, 0)

	var err error

	for _, url := range h.config.URLs {

		ir := goexpose.NewResponse(http.StatusOK).StripStatusData()

		client := &http.Client{}
		var (
			format   string
			req      *http.Request
			resp     *http.Response
			respbody []byte
			body     io.Reader
		)

		if url.PostBody && r.Body != nil {
			body = r.Body
		}

		method := r.Method

		// if method is given
		if url.Method != "" {
			method = url.Method
		}

		var b string
		if b, err = goexpose.Interpolate(url.URL, data); err != nil {
			ir.Error(err)
			goto Append
		}

		if req, err = http.NewRequest(method, b, body); err != nil {
			ir.Error(err)
			goto Append
		}

		if resp, err = client.Do(req); err != nil {
			ir.Error(err)
			goto Append
		}

		if respbody, err = ioutil.ReadAll(resp.Body); err != nil {
			ir.Error(err)
			goto Append
		}

		// prepare response
		ir.Status(resp.StatusCode)

		// return headers?
		if url.ReturnHeaders {
			ir.AddValue("headers", resp.Header)
		}

		// get format(if available)
		format = url.Format

		// try to guess json
		if !goexpose.HasFormat(format, "json") {
			ct := strings.ToLower(r.Header.Get("Content-Type"))
			if strings.Contains(ct, "application/json") {
				if !goexpose.HasFormat(format, "json") {
					format = goexpose.AddFormat(format, "json")
				}
			}
		}

		if re, f, e := goexpose.Format(string(respbody), url.Format); e == nil {
			ir.Result(re).AddValue("format", f)
		} else {
			ir.Error(e)
		}

	Append:
		results = append(results, ir)
	}

	// return single result
	if h.config.singleResultIndex != -1 {
		return response.Result(results[h.config.singleResultIndex])
	} else {
		return response.Result(results)
	}
}
