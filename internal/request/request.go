package request

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/amaan287/httpserver/internal/headers"
)

type parserState string

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

const (
	StateInit    parserState = "init"
	StateHeaders parserState = "headers"
	StateBody    parserState = "body"
	StateDone    parserState = "done"
	StateError   parserState = "error"
)

// chunkParseState tracks progress through a chunked-encoded body.
type chunkParseState string

const (
	chunkStateSize    chunkParseState = "size"
	chunkStateData    chunkParseState = "data"
	chunkStateTrailer chunkParseState = "trailer"
)

type Request struct {
	RequestLine RequestLine
	Headers     *headers.Headers
	Body        string
	state       parserState

	chunkState     chunkParseState
	chunkRemaining int
}

func newRequest() *Request {
	return &Request{
		state:      StateInit,
		Headers:    headers.NewHeaders(),
		Body:       "",
		chunkState: chunkStateSize,
	}

}
func (r *RequestLine) ValidHTTP() bool {
	return r.HttpVersion == "HTTP/1.1"
}

var ERROR_REQUEST_IN_ERROR_STATE = fmt.Errorf("request in error state")
var ERROR_BAD_START_LINE = fmt.Errorf("bad start line")
var ERROR_MALFORMED_REQUEST_LINE = fmt.Errorf("malformed request-line")
var ERROR_UNSUPPORTED_HTTP_VERSION = fmt.Errorf("Http version is not supported")
var ERROR_INCOMPLETE_REQUEST = fmt.Errorf("incomplete request: connection closed before request finished")

var SEPERATOR = []byte("\r\n")

// initialBufferSize is the read buffer's starting size; it doubles when full.
const initialBufferSize = 1024

func (r *Request) done() bool {
	return r.state == StateDone || r.state == StateError
}
func parseRequestLine(b []byte) (*RequestLine, int, error) {
	idx := bytes.Index(b, SEPERATOR)
	if idx == -1 {
		return nil, 0, nil
	}
	startLine := b[:idx]
	read := idx + len(SEPERATOR)
	parts := bytes.Split(startLine, []byte(" "))
	if len(parts) != 3 {
		return nil, 0, ERROR_MALFORMED_REQUEST_LINE
	}
	httpParts := bytes.Split(parts[2], []byte("/"))
	if len(httpParts) != 2 || string(httpParts[0]) != "HTTP" || string(httpParts[1]) != "1.1" {
		return nil, 0, ERROR_MALFORMED_REQUEST_LINE
	}
	rl := &RequestLine{
		Method:        string(parts[0]),
		RequestTarget: string(parts[1]),
		HttpVersion:   string(httpParts[1]),
	}
	return rl, read, nil

}
func getInt(header *headers.Headers, name string, defaultValue int) int {

	valueStr, exists := header.Get(name)
	if !exists {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func (r *Request) isChunked() bool {
	te, ok := r.Headers.Get("transfer-encoding")
	return ok && strings.Contains(strings.ToLower(te), "chunked")
}

func (r *Request) hasBody() bool {
	if getInt(r.Headers, "content-length", 0) > 0 {
		return true
	}
	return r.isChunked()
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0
outer:
	for {
		currentData := data[read:]
		if len(currentData) == 0 {
			break outer
		}
		switch r.state {
		case StateError:
			return 0, ERROR_REQUEST_IN_ERROR_STATE
		case StateInit:
			rl, n, err := parseRequestLine(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break outer
			}
			r.RequestLine = *rl
			read += n
			r.state = StateHeaders
		case StateHeaders:
			n, done, err := r.Headers.Parse(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break outer
			}
			read += n
			if done {
				if r.hasBody() {
					r.state = StateBody
				} else {
					r.state = StateDone
				}
			}
		case StateBody:
			n, err := r.parseBody(currentData)
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break outer
			}
			read += n
		case StateDone:
			break outer
		default:
			panic("somehow we are noob at programming")
		}
	}
	return read, nil
}

// parseBody consumes body bytes per the declared framing (chunked or Content-Length).
func (r *Request) parseBody(data []byte) (int, error) {
	if r.isChunked() {
		return r.parseChunkedBody(data)
	}

	contentLength := getInt(r.Headers, "content-length", 0)
	if contentLength <= 0 {
		// shouldn't happen if hasBody() is correct; avoid looping forever.
		r.state = StateDone
		return 0, nil
	}
	remaining := min(contentLength-len(r.Body), len(data))
	r.Body += string(data[:remaining])
	if len(r.Body) == contentLength {
		r.state = StateDone
	}
	return remaining, nil
}

// parseChunkedBody implements RFC 9112 chunked transfer-coding.
func (r *Request) parseChunkedBody(data []byte) (int, error) {
	read := 0
	for {
		switch r.chunkState {
		case chunkStateSize:
			idx := bytes.Index(data[read:], SEPERATOR)
			if idx == -1 {
				return read, nil
			}
			sizeLine := data[read : read+idx]
			if semi := bytes.IndexByte(sizeLine, ';'); semi != -1 {
				sizeLine = sizeLine[:semi] // ignore chunk extensions
			}
			size, err := strconv.ParseInt(string(bytes.TrimSpace(sizeLine)), 16, 64)
			if err != nil || size < 0 {
				return 0, fmt.Errorf("invalid chunk size: %q", sizeLine)
			}
			read += idx + len(SEPERATOR)
			r.chunkRemaining = int(size)
			if size == 0 {
				r.chunkState = chunkStateTrailer
			} else {
				r.chunkState = chunkStateData
			}
		case chunkStateData:
			available := len(data) - read
			if r.chunkRemaining > 0 {
				if available == 0 {
					return read, nil
				}
				n := min(r.chunkRemaining, available)
				r.Body += string(data[read : read+n])
				read += n
				r.chunkRemaining -= n
				available -= n
			}
			if r.chunkRemaining > 0 {
				return read, nil
			}
			if available < len(SEPERATOR) {
				return read, nil
			}
			if !bytes.Equal(data[read:read+len(SEPERATOR)], SEPERATOR) {
				return 0, fmt.Errorf("malformed chunk data terminator")
			}
			read += len(SEPERATOR)
			r.chunkState = chunkStateSize
		case chunkStateTrailer:
			n, done, err := r.Headers.Parse(data[read:])
			if err != nil {
				return 0, err
			}
			if n == 0 {
				return read, nil
			}
			read += n
			if done {
				r.state = StateDone
				return read, nil
			}
		}
	}
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := newRequest()
	buf := make([]byte, initialBufferSize)
	bufLen := 0
	for !request.done() {
		if bufLen == len(buf) {
			grown := make([]byte, len(buf)*2)
			copy(grown, buf)
			buf = grown
		}
		n, err := reader.Read(buf[bufLen:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				// process remaining buffer before exiting
				if bufLen > 0 {
					readN, parseErr := request.parse(buf[:bufLen])
					if parseErr != nil {
						return nil, parseErr
					}
					bufLen -= readN
				}
				if !request.done() {
					return nil, ERROR_INCOMPLETE_REQUEST
				}
				break
			}
			return nil, err
		}
		bufLen += n
		readN, err := request.parse(buf[:bufLen])
		if err != nil {
			return nil, err
		}
		copy(buf, buf[readN:bufLen])
		bufLen -= readN
	}
	return request, nil
}
