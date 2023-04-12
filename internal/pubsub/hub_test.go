package pubsub

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPubsub_NewSession(t *testing.T) {
	ps, err := New()
	assert.Nil(t, err)

	ss, err := ps.NewSession()
	assert.Nil(t, err)

	assert.NotNil(t, ss)

}
