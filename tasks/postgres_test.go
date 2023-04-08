package tasks

import (
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/domain/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPostgresTaskConfig_Validate(t *testing.T) {

	t.Run("invalid config", func(t *testing.T) {

		for _, item := range []struct {
			data          map[string]any
			expectedError []string
		}{
			{
				data: map[string]any{"queries": []map[string]any{
					{
						"url": "",
					},
				}},
				expectedError: []string{
					domain.ErrMissingURL.Error(),
				},
			},
			{
				data: map[string]any{"queries": []map[string]any{
					{
						"url": "localhost",
					},
				}},
				expectedError: []string{
					domain.ErrInvalidQuery.Error(),
				},
			},
		} {
			_, err := PostgresTaskInitFunc(mocks.NewServer(t), taskConfigFrom(t, item.data), &domain.EndpointConfig{})
			assert.NotNil(t, err)
			for _, expectedError := range item.expectedError {
				assert.Contains(t, err.Error(), expectedError)
			}
		}

	})

}
