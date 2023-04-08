package goexpose

/*
Response module

small helper to make writing json responses easier with along with logging of requests.

Usage:

	in all examples r is *http.Request and w is http.ResponseWriter
	NewResponse(http.StatusNotFound).Write(w, r)

	writes following json response along with log message:
	{
		"status": 404,
		"message": "Not found"
	}

	Following example adds also time to response log, and also result.
	Result will be marshalled json

	t := time.Now()
	NewResponse(http.StatusOK).Response(map[string]interface{}{}).Write(w, r, t)

	yields to
	{
		"status": 200,
		"message": "OK",
		"result": {}
	}

	In case we want to add another key in top level structure we can do following

	NewResponse(http.StatusOK).AddValue("size", 1).Write(w, r)
	yields to:

	{
		"status": 200,
		"message": "OK",
		"size": 1
	}

	We have also shorthand method for error

	NewResponse(http.StatusInternalServerError).Error("error").Write(w, r)

	yields to:

	{
		"status": 500,
		"message": "Internal Server Error",
		"error": "error"
	}

	For responses that don't need status/message in json response, you need to call
	StripStatusData method. This is useful for embedded Responses inside Responses.
	So e.g.:

	NewResponse(http.StatusOK).StripStatusData()

	will yield to:

	{}
*/

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"encoding/base64"

	"github.com/golang/glog"
)

/*
Creates new json response
*/
func NewResponse(status int) (response *Response) {
	response = &Response{
		data: map[string]interface{}{},
	}
	response.Status(status)
	return
}

/*
Response
*/
type Response struct {
	status int
	pretty bool

	// json data
	data map[string]interface{}

	// raw data
	raw *[]byte
}

/*
Set status
*/
func (r *Response) Status(status int) *Response {
	r.status = status
	return r.AddValue("status", status).AddValue("message", http.StatusText(r.status))
}

/*
GetStatus returns status
*/
func (r *Response) GetStatus() int {
	return r.status
}

/*
Sets pretty printing of json
*/
func (r *Response) Pretty(pretty bool) *Response {
	r.pretty = pretty
	return r
}

/*
Result method adds result, it's just a shorthand to AddValue("result", result)
*/
func (r *Response) Result(result interface{}) *Response {
	return r.AddValue("result", result)
}

/*
Raw Sets raw response
param can be following:

	nil => clear raw
	fmt.Stringer or string => convert to []byte
	[]byte leave as it is
	otherwise try to marshal to json []byte
*/
func (r *Response) Raw(raw interface{}) *Response {
	if raw == nil {
		r.raw = nil
		return r
	}
	switch t := raw.(type) {
	case fmt.Stringer:
		resp := []byte(t.String())
		r.raw = &resp
	case string:
		resp := []byte(t)
		r.raw = &resp
	case []byte:
		r.raw = &t
	default:
		var (
			body []byte
			err  error
		)
		if body, err = json.Marshal(raw); err == nil {
			r.raw = &body
		} else {
			r.Error(err)
		}
	}

	return r
}

/*
Error method adds error, it's just a shorthand to AddValue("error", err)

@TODO: store just string from error
*/
func (r *Response) Error(err interface{}) *Response {
	return r.AddValue("error", err)
}

/*
Adds value
*/
func (r *Response) AddValue(key string, value interface{}) *Response {
	r.data[key] = value
	return r
}

/*
Removes value
*/
func (r *Response) DelValue(key string) *Response {
	delete(r.data, key)
	return r
}

/*
Whether response has value
*/
func (r *Response) HasValue(key string) bool {
	_, ok := r.data[key]
	return ok
}

/*
Writes response to response writer and logs request
*/
func (r *Response) Write(w http.ResponseWriter, req *http.Request, start ...time.Time) (err error) {
	var (
		body []byte
	)

	// add headers
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(r.status)

	// write body
	if r.raw != nil {
		w.Write(*r.raw)
	} else {
		if body, err = json.Marshal(r); err != nil {
			return
		}
		w.Write(body)
	}

	var (
		format string
		args   []interface{}
	)

	// log request
	if len(start) > 0 {
		format = "%s %s %d %v"
		args = []interface{}{req.Method, req.URL.Path, r.status, time.Now().Sub(start[0])}
	} else {
		format = "%s %s %d"
		args = []interface{}{req.Method, req.URL.Path, r.status}
	}

	glog.V(1).Infof(format, args...)

	return
}

/*
Marshaler interface
support json marshalling
*/
func (r *Response) MarshalJSON() (result []byte, err error) {
	if r.raw != nil {
		buf := make([]byte, base64.StdEncoding.EncodedLen(len(*r.raw)))
		base64.StdEncoding.Encode(buf, *r.raw)
		return buf, nil
	}
	if r.pretty {
		result, err = json.MarshalIndent(r.data, "", "    ")
	} else {
		result, err = json.Marshal(r.data)
	}

	return
}

/*
Strips status/message from data
*/
func (r *Response) StripStatusData() *Response {
	delete(r.data, "status")
	if message, ok := r.data["message"]; ok {
		if message == http.StatusText(r.status) {
			delete(r.data, "status")
		}
	}

	return r
}

/*
Updates stripped status data
*/
func (r *Response) UpdateStatusData() *Response {
	return r.Status(r.status)
}
