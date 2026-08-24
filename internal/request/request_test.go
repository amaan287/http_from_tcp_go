package request

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chunkReader struct {
	data            string
	numBytesPerRead int
	pos             int //pos = current position
}

func (cr *chunkReader) Read(p []byte) (n int, err error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	endIndex := min(cr.pos+cr.numBytesPerRead, len(cr.data))
	n = copy(p, cr.data[cr.pos:endIndex])
	cr.pos += n
	if n > cr.numBytesPerRead {
		n = cr.numBytesPerRead
		cr.pos -= n - cr.numBytesPerRead
	}
	return n, nil

}

func TestRequestLineParse(t *testing.T) {
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHOST: localhost:42069\r\nUser-Agent: curl/8.5.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)

	r, err = RequestFromReader(strings.NewReader("GET /coffee HTTP/1.1\r\nHOST: localhost:42069\r\nUser-Agent: curl/8.5.0\r\nAccept: */*\r\n\r\n"))
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/coffee", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)

	_, err = RequestFromReader(strings.NewReader("/coffee HTTP/1.1\r\nHOST:localhost:42069\r\nUser-Agent: curl/8.5.0\r\nAccept: */*\r\n\r\n"))
}

func TestParseHeaders(t *testing.T) {
	reader := &chunkReader{
		data:            "GET / HTTP/1.1\r\nHOST: localhost:42069\r\nUser-Agent: curl/8.5.0\r\nAccept: */*\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	host, ok := r.Headers.Get("Host")
	assert.True(t, ok)
	assert.Equal(t, "localhost:42069", host)
	host, ok = r.Headers.Get("user-agent")
	assert.True(t, ok)
	assert.Equal(t, "curl/8.5.0", host)
	host, ok = r.Headers.Get("accept")
	assert.True(t, ok)
	assert.Equal(t, "*/*", host)
	reader = &chunkReader{
		data:            "GET / HTTP/1.1\r\nHost localhost:42069\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
}

func TestParseBody(t *testing.T) {
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"Content-Length: 13\r\n" +
			"\r\n" +
			"hello world!\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "hello world!\n", string(r.Body))

	reader = &chunkReader{
		data: "Content-Length: 13\r\n" +
			"Host: localhost:42069\r\n" +
			"Content-Length: 20\r\n" +
			"\r\n" +
			"partial content",
		numBytesPerRead: 3,
	}
	r, err = RequestFromReader(reader)
	require.Error(t, err)
}

// Regression test: a body-less request must reach StateDone, or a pipelined follow-up gets misread as body.
func TestNoBodyRequest(t *testing.T) {
	reader := &chunkReader{
		data: "GET / HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"\r\n" +
			"GET /second HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/", r.RequestLine.RequestTarget)
	assert.Equal(t, "", r.Body)
}

func TestParseChunkedBody(t *testing.T) {
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"4\r\nWiki\r\n" +
			"5\r\npedia\r\n" +
			"E\r\n in\r\n\r\nchunks.\r\n" +
			"0\r\n\r\n",
		numBytesPerRead: 3,
	}
	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Wikipedia in\r\n\r\nchunks.", r.Body)
}

func TestIncompleteRequest(t *testing.T) {
	// Content-Length promises 20 bytes but only 7 arrive before EOF.
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: localhost:42069\r\n" +
			"Content-Length: 20\r\n" +
			"\r\n" +
			"partial",
		numBytesPerRead: 3,
	}
	_, err := RequestFromReader(reader)
	require.ErrorIs(t, err, ERROR_INCOMPLETE_REQUEST)
}
