package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJsonBytes_MarshalJSON(t *testing.T) {

	t.Run("complete lifecycle", func(t *testing.T) {
		var src = []byte("hello world")
		var j JsonBytes = src

		b, err := j.MarshalJSON()
		assert.Nil(t, err)

		var dst JsonBytes
		err = dst.UnmarshalJSON(b)
		assert.Nil(t, err)

		assert.Equal(t, src, []byte(dst))
	})

}

func TestJsonStringSlice_UnmarshalJSON(t *testing.T) {

	t.Run("single string", func(t *testing.T) {
		var src = []byte("\"hello world\"")
		var dst JsonStringSlice

		err := dst.UnmarshalJSON(src)
		assert.Nil(t, err)

		assert.Equal(t, []string{"hello world"}, []string(dst))
	})

	t.Run("slice of strings", func(t *testing.T) {
		var src = []byte("[\"hello world\", \"hello world\"]")
		var dst JsonStringSlice

		err := dst.UnmarshalJSON(src)
		assert.Nil(t, err)

		assert.Equal(t, []string{"hello world", "hello world"}, []string(dst))
	})
}
