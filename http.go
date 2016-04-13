package goexpose

import (
	"io"
	"net"
	"net/http"
	"time"
)

/*
Various http helpers and utilities

Requester
makes http requests
*/

const (
	DEFAULT_TIMEOUT = 10 * time.Second
)

type RequesterSetFunc func(r *Requester)

func NewRequester(funcs ...RequesterSetFunc) (result *Requester) {
	result = &Requester{}
	result.Set(WithTimeout(DEFAULT_TIMEOUT))

	if len(funcs) > 0 {
		result.Set(funcs...)
	}

	return
}

/*
Making requests
*/
type Requester struct {
	timeout time.Duration
	client  *http.Client
}

/*
Do performs request and returns response or error
*/
func (r *Requester) DoRequest(req *http.Request) (resp *http.Response, err error) {
	return r.client.Do(req)
}

/*
DoNew creates new request and sends it
*/
func (r *Requester) DoNew(method string, url string, body io.Reader) (req *http.Request, resp *http.Response, err error) {

	if req, err = http.NewRequest(method, url, body); err != nil {
		return
	}

	resp, err = r.client.Do(req)
	return
}

/*
With is used to change values directly from constructors
*/
func (r *Requester) Set(funcs ...RequesterSetFunc) *Requester {
	for _, fn := range funcs {
		fn(r)
	}
	return r
}

/*
Set functions
*/
func WithTimeout(timeout time.Duration) RequesterSetFunc {
	return func(r *Requester) {
		r.timeout = timeout
		r.client = &http.Client{
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout: r.timeout,
					//KeepAlive: 30 * time.Second,
				}).Dial,
				//TLSHandshakeTimeout: secs * time.Second,
			},
		}
	}
}
