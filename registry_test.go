package goexpose

import (
	"github.com/phonkee/goexpose/domain"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func ExampleTaskFactory(server domain.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {
	tasks = []domain.Task{}
	return
}

func TestRegistry(t *testing.T) {

	Convey("Test Add", t, func() {
		RegisterTaskFactory("example", ExampleTaskFactory)
		So(func() { RegisterTaskFactory("example", ExampleTaskFactory) }, ShouldPanic)

		tf, ok := GetTaskFactory("example")
		So(ok, ShouldBeTrue)
		So(tf, ShouldEqual, tf)

		_, ok = GetTaskFactory("nonexisting")
		So(ok, ShouldBeFalse)
	})

}
