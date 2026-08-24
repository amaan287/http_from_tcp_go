package response

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteStatusLine(t *testing.T) {
	buf := &bytes.Buffer{}
	require.NoError(t, WriteStatusLine(buf, StatusOK))
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", buf.String())
}

func TestWriteHeaders(t *testing.T) {
	buf := &bytes.Buffer{}
	h := GetDefaultHeaders(13)
	require.NoError(t, WriteHeaders(buf, h))

	// Headers lower-cases names; still valid since HTTP names are case-insensitive.
	out := buf.String()
	assert.Contains(t, out, "content-length: 13\r\n")
	assert.Contains(t, out, "connection: close\r\n")
	assert.True(t, bytes.HasSuffix(buf.Bytes(), []byte("\r\n\r\n")))
}

func TestWriteBody(t *testing.T) {
	buf := &bytes.Buffer{}
	require.NoError(t, WriteBody(buf, []byte("hello world!\n")))
	assert.Equal(t, "hello world!\n", buf.String())
}
