package goexpose

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func ExampleTaskFactory(server *Server, taskconfig *TaskConfig, ec *EndpointConfig) (tasks []Tasker, err error) {
	tasks = []Tasker{}
	return
}

func TestRegistry(t *testing.T) {

	Convey("Test Add", t, func() {
		RegisterTaskFactory("example", ExampleTaskFactory)
		So(func() { RegisterTaskFactory("example", ExampleTaskFactory) }, ShouldPanic)

		tf, ok := getTaskFactory("example")
		So(ok, ShouldBeTrue)
		So(tf, ShouldEqual, tf)

		_, ok = getTaskFactory("nonexisting")
		So(ok, ShouldBeFalse)
	})

}
