package goexpose

import (
	"testing"
	"time"

	"io"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRequester(t *testing.T) {

	Convey("Test Set function `WithTimeout`", t, func() {
		var tdata = []struct {
			timeout time.Duration
		}{
			{311 * time.Second},
			{12 * time.Second},
			{2 * time.Second},
			{333 * time.Second},
		}

		for _, item := range tdata {
			requester := NewRequester(WithTimeout(item.timeout))
			So(requester.timeout, ShouldEqual, item.timeout)
		}
	})

	Convey("Test DoNew", t, func() {
		var tdata = []struct {
			method  string
			url     string
			body    io.Reader
			timeout time.Duration
		}{
			{"GET", "http://www.google.com", nil, 1 * time.Millisecond},
		}

		for _, item := range tdata {
			requester := NewRequester(WithTimeout(item.timeout))
			request, response, err := requester.DoNew(item.method, item.url, item.body)
			So(err, ShouldNotBeNil)
			So(request.Method, ShouldEqual, item.method)
			So(request.URL.String(), ShouldEqual, item.url)
			So(response, ShouldBeNil)
		}
	})

	Convey("Test DoNew invalid", t, func() {
		var tdata = []struct {
			method string
			url    string
			body   io.Reader
		}{
			{"GET", "hehehehe", nil},
			{"GET", "eee.adsffsddf.cz", nil},
			{"ASDF", "", nil},
		}

		for _, item := range tdata {
			requester := NewRequester()
			So(requester, ShouldNotBeNil)
			req, resp, err := requester.DoNew(item.method, item.url, item.body)
			_, _ = req, resp
			So(err, ShouldNotBeNil)
		}
	})

}
