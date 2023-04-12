package tests

import (
	"context"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/pubsub"
	"github.com/phonkee/goexpose/internal/pubsub/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestPubSubRequest_Execute(t *testing.T) {

	t.Run("test empty", func(t *testing.T) {
		sess := &mocks.Session{}
		psr := &pubsub.Request{}
		resp, err := psr.Execute(context.Background(), sess)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), domain.ErrPubSubInvalidRequest.Error())
		assert.Nil(t, resp)
	})

	t.Run("test non null requests", func(t *testing.T) {
		for _, item := range []struct {
			setup   func(session *mocks.Session)
			request *pubsub.Request
		}{
			{
				setup: func(session *mocks.Session) {
					session.On("Publish", mock.Anything, mock.Anything).Return(nil)
				},
				request: &pubsub.Request{
					Publish: &pubsub.RequestPublish{
						Channel: []string{"channel"},
						Message: []byte("message"),
					},
				},
			},
			{
				setup: func(session *mocks.Session) {
					session.On("Subscribe", mock.Anything, mock.Anything).Return(nil)
				},
				request: &pubsub.Request{
					Subscribe: &pubsub.RequestSubscribe{
						Channel: []string{"channel"},
					},
				},
			},
			{
				setup: func(session *mocks.Session) {
					session.On("UnSubscribe", mock.Anything, mock.Anything).Return(nil)
				},
				request: &pubsub.Request{
					UnSubscribe: &pubsub.RequestUnSubscribe{
						Channel: []string{"channel"},
					},
				},
			},
			{
				setup: func(session *mocks.Session) {
					session.On("Subscriptions", mock.Anything, mock.Anything).Return(nil)
				},
				request: &pubsub.Request{
					Subscriptions: &pubsub.RequestSubscriptions{},
				},
			},
		} {
			sess := &mocks.Session{}
			if item.setup != nil {
				item.setup(sess)
			}

			resp, err := item.request.Execute(context.Background(), sess)
			assert.Nil(t, err)
			assert.NotNil(t, resp)

			sess.AssertExpectations(t)
		}
	})

}
