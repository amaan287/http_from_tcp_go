package request

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"

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

type Request struct {
	RequestLine RequestLine
	Headers     *headers.Headers
	Body        string
	state       parserState
}

func newRequest() *Request {
	return &Request{
		state:   StateInit,
		Headers: headers.NewHeaders(),
		Body:    "",
	}

}
func (r *RequestLine) ValidHTTP() bool {
	return r.HttpVersion == "HTTP/1.1"
}

var ERROR_REQUEST_IN_ERROR_STATE = fmt.Errorf("request in error state")
var ERROR_BAD_START_LINE = fmt.Errorf("bad start line")
var ERROR_MALFORMED_REQUEST_LINE = fmt.Errorf("malformed request-line")
var ERROR_UNSUPPORTED_HTTP_VERSION = fmt.Errorf("Http version is not supported")

var SEPERATOR = []byte("\r\n")

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

func (r *Request) hasBody() bool {
	//TODO: when doing chunked encoding updated this method
	length := getInt(r.Headers, "Content-length", 0)
	return length == 0
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
				r.state = StateBody
			}
		case StateBody:
			lengthStr := getInt(r.Headers, "content-length", 0)
			if lengthStr == 0 {
				panic("Chunked not implemented")
			}
			remaining := min(lengthStr-len(r.Body), len(currentData))
			r.Body += string(currentData[:remaining])
			read += remaining
			slog.Info("parse state Body", "remaining", remaining, "read", read, "body", r.Body)
			if len(r.Body) == lengthStr {
				r.state = StateDone
			}
		case StateDone:
			break outer
		default:
			panic("somehow we are noob at programming")
		}
	}
	return read, nil
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := newRequest()
	// NOTE: buffer could overrun... a header that exceeds 1k would do that...
	// or body
	buf := make([]byte, 1024)
	bufLen := 0
	for !request.done() {
		n, err := reader.Read(buf[bufLen:])
		//TODO handle io.EOF error
		if err != nil {
			if errors.Is(err, io.EOF) {
				// process remaining buffer before exiting
				if bufLen > 0 {
					_, parseErr := request.parse(buf[:bufLen])
					if parseErr != nil {
						return nil, parseErr
					}
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
