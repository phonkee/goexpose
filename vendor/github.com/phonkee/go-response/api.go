package response

import "net/http"

const (
	statusKey = 118999
)

/*
New returns new response instance, if no status is given StatusOK is used
*/
func New(statuses ...int) (result Response) {
	result = &response{
		data:    map[string]interface{}{},
		headers: map[string]string{},
	}
	result.ContentType("application/json")
	if len(statuses) > 0 {
		result.Status(statuses[0])
	} else {
		result.Status(http.StatusOK)
	}
	return
}

/*
Body is helper to create response
*/
func Body(body interface{}) Response {
	return New().Body(body)
}

/*
Data is helper to create status ok response.
*/
func Data(key string, value interface{}) Response {
	return New().Data(key, value)
}

/*
Error is helper to create status ok response.
*/
func Error(err interface{}) Response {
	return New(http.StatusInternalServerError).Error(err)
}

/*
HTML returns response set to HTML
*/
func HTML(html string) Response {
	return New().HTML(html)
}

/*
Result is helper to create status ok response.
*/
func Result(result interface{}) Response {
	return New().Result(result)
}

/*
SliceResult is helper to create status ok response.
*/
func SliceResult(result interface{}) Response {
	return New().SliceResult(result)
}

/*
Write writes to response and returns error
*/
func Write(w http.ResponseWriter, r *http.Request) {
	New().Write(w, r)
}

/*
Couple of shorthands to return responses with common http statuses
*/

/*
BadRequest returns response with StatusBadRequest
*/
func BadRequest() Response {
	return New(http.StatusBadRequest)
}

/*
NotFound returns response with StatusNotFound
*/
func NotFound() Response {
	return New(http.StatusNotFound)
}

/*
OK returns response with StatusOK
*/
func OK() Response {
	return New()
}

/*
Unauthorized returns response with StatusUnauthorized
*/
func Unauthorized() Response {
	return New(http.StatusUnauthorized)
}
