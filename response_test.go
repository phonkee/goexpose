package goexpose

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
)

func TestResponse(t *testing.T) {

	Convey("Test New response", t, func() {
		response := NewResponse(http.StatusOK)
		So(response.status, ShouldEqual, http.StatusOK)
		So(response.data["status"].(int), ShouldEqual, http.StatusOK)

		// test pretty
		So(response.pretty, ShouldBeFalse)
		So(response.Pretty(true).pretty, ShouldBeTrue)
		So(response.Pretty(false).pretty, ShouldBeFalse)

		// test result
		So(response.Result(1).data["result"].(int), ShouldEqual, 1)

		// add value/del value
		So(response.AddValue("some", "value").data["some"].(string), ShouldEqual, "value")
		So(response.DelValue("some").data["some"], ShouldBeNil)

		// check error
		So(response.Error("fatal").data["error"].(string), ShouldEqual, "fatal")
	})

}
