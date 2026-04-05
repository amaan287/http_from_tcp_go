package headers

import (
	"bytes"
	"fmt"
	"strings"
)

var rn = []byte("\r\n")

func isToken(str []byte) bool {
	for _, ch := range str {
		found := false
		if ch >= 'A' && ch <= 'Z' ||
			ch >= 'a' && ch <= 'z' ||
			ch >= '0' && ch <= '9' {
			found = true
		}
		switch ch {
		case '!', '#', '$', '%', '&', '\'', '*', '-', '.', '^', '_', '`', '|', '~':
			found = true
		}
		if !found {
			return false
		}
	}
	return true
}
func parseHeader(fieldLine []byte) (string, string, error) {
	idx := bytes.IndexByte(fieldLine, ':')
	if idx == -1 {
		return "", "", fmt.Errorf("malformed field line")
	}

	name := fieldLine[:idx]
	value := fieldLine[idx+1:]

	if bytes.Contains(name, []byte(" ")) {
		return "", "", fmt.Errorf("malformed field name")
	}

	name = bytes.TrimSpace(name)
	value = bytes.TrimSpace(value)

	if len(name) == 0 {
		return "", "", fmt.Errorf("empty header name")
	}

	return string(name), string(value), nil
}

type Headers struct {
	headers map[string]string
}

func NewHeaders() *Headers {
	return &Headers{
		headers: map[string]string{},
	}
}

func (h *Headers) Get(name string) (string, bool) {
	str, ok := h.headers[strings.ToLower(name)]
	return str, ok
}

func (h *Headers) Set(name, value string) {
	name = strings.ToLower(name)
	if v, ok := h.headers[name]; ok {
		h.headers[name] = fmt.Sprintf("%s,%s", v, value)
	} else {
		h.headers[name] = value
	}
}
func (h *Headers) ForEach(cb func(n, v string)) {
	for n, v := range h.headers {
		cb(n, v)
	}
}
func (h *Headers) Parse(data []byte) (int, bool, error) {
	read := 0
	done := false
	for {
		idx := bytes.Index(data[read:], rn)
		if idx == -1 {
			break
		}
		//empty header
		if idx == 0 {
			done = true
			read += len(rn)
			break
		}
		name, value, err := parseHeader(data[read : read+idx])
		if err != nil {
			return 0, false, err
		}
		read += idx + len(rn)
		h.Set(name, value)
	}
	return read, done, nil
}
