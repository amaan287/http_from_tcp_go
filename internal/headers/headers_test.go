package headers

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHeadeParse(t *testing.T) {
	headers := NewHeaders()
	data := []byte("Host: localhost:42069\r\nFooFoo:      barbar      \r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	host, ok := headers.Get("HOST")
	assert.True(t, ok)
	assert.Equal(t, "localhost:42069", host)
	host, ok = headers.Get("FooFoo")
	assert.True(t, ok)
	assert.Equal(t, "barbar", host)
	_, ok = headers.Get("MissingKey")
	assert.False(t, ok)
	assert.Equal(t, 52, n)
	assert.True(t, done)

	// Test: Invalid spacing Header
	headers = NewHeaders()
	data = []byte("       H@st : localhost:42069       \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nHost: localhost:42069\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	host, ok = headers.Get("HOST")
	assert.True(t, ok)
	assert.Equal(t, "localhost:42069,localhost:42069", host)
	assert.False(t, done)
}
