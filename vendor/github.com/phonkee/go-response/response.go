/*
Response
helper interface for structured json responses. Response has also support to provide HTML content, but primarily it's

targeted to json rest apis

Response supports errors, raw content, etc..

Setter methods support chaining so writing response to http is doable on single line
In next examples we use these variables

	w http.ResponseWriter
	r *http.Request

Example of responses

	response.New(http.StatusInternalServerError).Error(errors.New("error")).Write(w, r)
	response.New().Error(structError).Write(w, r)
	response.New().Result(product).Write(w, r)
	response.New().Result(products).ResultSize(size).Write(w, r)
	response.New().SliceResult(products).Write(w, r)
	response.New(http.StatusForbidden).Write(w, r)

Also there is non required argument status

	body := map[string]string{
		"version": "1.0beta"
	}
	response.New(http.StatusOK).Body(body)Write(w, r)
	response.New(http.StatusOK).Result(product).Write(w, r)
	response.New(http.StatusOK).SliceResult(products).Write(w, r)

Minimal support for html responses

	response.New().HTML("<html></html>").Write(w, r)
*/
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
)

const STATUS_HEADER = "X-Response-Status"

/*
Response interface provides several method for Response
*/
type Response interface {

	// set raw body
	Body(body interface{}) Response

	// Set Content type
	ContentType(contenttype string) Response

	// set data to response data
	Data(key string, value interface{}) Response

	// delete data value identified by key
	DeleteData(key string) Response

	// delete header by name
	DeleteHeader(name string) Response

	// sets `error` to response data
	Error(err interface{}) Response

	// returns response as []byte
	GetBytes() []byte

	HasData(key string) bool
	// set header with value
	Header(name, value string) Response

	// set html and content type (for template rendering)
	HTML(html string) Response

	// set http message
	Message(message string) Response

	// custom json marshalling function
	MarshalJSON() (result []byte, err error)

	// set `result` on response data (shorthand for Data("result", data))
	Result(result interface{}) Response

	// set result_size on response
	ResultSize(size int) Response

	// set slice result (sets `result` and `result_size`)
	SliceResult(result interface{}) Response

	// set http status
	Status(status int) Response

	// return string value of response
	String() (body string)

	// Write complete response to writer
	Write(w http.ResponseWriter, request *http.Request)
}

/*
response is implementation of Response interface
*/
type response struct {
	contentType string
	data        map[string]interface{}
	headers     map[string]string
	status      int
	message     string
	// Raw body
	body *string
}

/*
ContentType

	Set content type fo response
*/
func (r *response) ContentType(ct string) Response {
	r.contentType = ct
	return r
}

/*
ContentType

	Set content type fo response
*/
func (r *response) HTML(html string) Response {
	r.ContentType("text/html")
	r.body = &html
	return r
}

/*
Result set json result to response
*/
func (r *response) Result(result interface{}) Response {
	return r.Data(currentKeyFormat.ResultKey, result)
}

/*
ResultSize

	Set result_size to response
*/
func (r *response) ResultSize(size int) Response {
	return r.Data(currentKeyFormat.ResultSizeKey, size)
}

/*
Result

	Set result to response
*/
func (r *response) SliceResult(result interface{}) Response {
	r.Data(currentKeyFormat.ResultKey, result)
	value := reflect.ValueOf(result)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	if value.Kind() != reflect.Slice {
		return r
	}

	r.ResultSize(value.Len())
	return r
}

/*
Status

	set http status
*/
func (r *response) Status(status int) Response {
	r.status = status
	r.Message(http.StatusText(r.status))
	return r
}

/*
Data

	sets data on response
*/
func (r *response) Data(key string, value interface{}) Response {
	r.data[key] = value
	return r
}

/*
DelData

	removes data from response
*/
func (r *response) DeleteData(key string) Response {
	delete(r.data, key)
	return r
}

func (r *response) HasData(key string) bool {
	_, ok := r.data[key]
	return ok
}

/*
Error

	adds error to result
*/
func (r *response) Error(err interface{}) Response {
	switch err := err.(type) {
	case error:
		// set status from errMap
		if s := GetErrorStatus(err); s != 0 {
			r.Status(s)
		}
		r.data[currentKeyFormat.ErrorKey] = err.Error()
	case fmt.Stringer:
		r.data[currentKeyFormat.ErrorKey] = err.String()
	default:
		r.data[currentKeyFormat.ErrorKey] = err
	}
	return r
}

/*
Header

	sets header to response
*/
func (r *response) Header(name, value string) Response {
	r.headers[name] = value
	return r
}

/*
DeleteHeader

	deletes header value
*/
func (r *response) DeleteHeader(name string) Response {
	delete(r.headers, name)
	return r
}

/*
Message

	sets message
*/
func (r *response) Message(message string) Response {
	r.message = message
	return r
}

/*
Body

	sets raw body of response
	If value is stringer or string or []byte we will use string value.
	Otherwise we marshal it as json value
*/
func (r *response) Body(value interface{}) Response {
	switch v := value.(type) {
	case fmt.Stringer:
		tmp := v.String()
		r.body = &tmp
	case string:
		r.body = &v
	case nil:
		r.body = nil
	case []byte:
		tmp := string(v)
		r.body = &tmp
	default:
		if json, err := json.Marshal(value); err == nil {
			tmp := string(json)
			r.body = &tmp
		} else {
			r.body = nil
			return r.Error(err).Status(http.StatusInternalServerError)
		}
	}
	return r
}

// returns string representation
func (r *response) String() (body string) {
	return string(r.GetBytes())
}

// returns string representation
func (r *response) GetBytes() (body []byte) {
	if r.body == nil {
		var (
			b   []byte
			err error
		)
		// marshal self
		if b, err = json.Marshal(r); err == nil {
			body = b
		}
	} else {
		body = []byte(*r.body)
	}
	return
}

/*
MarshalJSON

	custom json marshallization
*/
func (r *response) MarshalJSON() (result []byte, err error) {
	data := map[string]interface{}{}
	for k, v := range r.data {
		data[k] = v
	}

	// add data to data
	data[currentKeyFormat.StatusKey] = r.status
	data[currentKeyFormat.MessageKey] = r.message

	result, err = json.Marshal(data)

	return
}

/*
Write

	writes response to writer
*/
func (r *response) Write(w http.ResponseWriter, request *http.Request) {

	// write headers
	w.Header().Set("Content-Type", r.contentType)
	for k, v := range r.headers {
		w.Header().Set(k, v)
	}

	// if not status set we set from context
	if r.status == 0 {
		if _, ok := r.data[currentKeyFormat.ErrorKey]; ok {
			r.status = http.StatusInternalServerError
		} else {
			r.status = http.StatusOK
		}
		// set status to response
		r.Status(r.status)
	}

	// the only way how to set status
	w.Header().Set(STATUS_HEADER, strconv.Itoa(r.status))

	// write status
	w.WriteHeader(r.status)
	fmt.Fprint(w, r.String())
	return
}
