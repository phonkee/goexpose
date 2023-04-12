package pubsub

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSession(t *testing.T) {

	t.Run("Test ID", func(t *testing.T) {
		sess := newSession(nil, nil)
		assert.NotZero(t, sess.ID())
		assert.Nil(t, sess.Subscribe(context.Background(), "what"))

		assert.Equal(t, len(sess.Subscriptions(context.Background())), 1)
		assert.Nil(t, sess.Subscribe(context.Background(), "else"))
		assert.Equal(t, len(sess.Subscriptions(context.Background())), 2)
	})

}
