package domain

import (
	"net/http"
	"time"
)

type Response interface {
	Status(status int) Response
	GetStatus() int
	Pretty(pretty bool) Response
	Result(result interface{}) Response
	Raw(raw interface{}) Response
	Error(err interface{}) Response
	AddValue(key string, value interface{}) Response
	DelValue(key string) Response
	HasValue(key string) bool
	Write(w http.ResponseWriter, req *http.Request, start ...time.Time) (err error)
	MarshalJSON() (result []byte, err error)
	StripStatusData() Response
	UpdateStatusData() Response
}
