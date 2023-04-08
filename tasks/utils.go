package tasks

import (
	"encoding/json"
	"github.com/phonkee/goexpose/domain"
	"testing"
)

// taskConfigFrom creates task config from any value (json)
func taskConfigFrom(t *testing.T, value any, fns ...func(c *domain.TaskConfig)) *domain.TaskConfig {
	t.Helper()

	what, err := json.Marshal(value)

	if err != nil {
		t.Fatalf("cannot marshal value: %s", err)
	}

	result := &domain.TaskConfig{
		Config: what,
	}

	for _, fn := range fns {
		fn(result)
	}

	return result

}
