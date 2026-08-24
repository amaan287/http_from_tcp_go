// Package response writes HTTP/1.1 responses to an io.Writer.
package response

import (
	"fmt"
	"io"
	"strconv"

	"github.com/amaan287/httpserver/internal/headers"
)

type StatusCode int

const (
	StatusOK                  StatusCode = 200
	StatusBadRequest          StatusCode = 400
	StatusNotFound            StatusCode = 404
	StatusInternalServerError StatusCode = 500
)

var reasonPhrases = map[StatusCode]string{
	StatusOK:                  "OK",
	StatusBadRequest:          "Bad Request",
	StatusNotFound:            "Not Found",
	StatusInternalServerError: "Internal Server Error",
}

// WriteStatusLine writes the "HTTP/1.1 <code> <reason>\r\n" line.
func WriteStatusLine(w io.Writer, statusCode StatusCode) error {
	_, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", statusCode, reasonPhrases[statusCode])
	return err
}

// GetDefaultHeaders returns the standard headers for a body of contentLen bytes.
func GetDefaultHeaders(contentLen int) *headers.Headers {
	h := headers.NewHeaders()
	h.Set("Content-Length", strconv.Itoa(contentLen))
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/plain")
	return h
}

// WriteHeaders writes each header followed by the blank line that terminates the block.
func WriteHeaders(w io.Writer, h *headers.Headers) error {
	var err error
	h.ForEach(func(n, v string) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, "%s: %s\r\n", n, v)
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\r\n")
	return err
}

// WriteBody writes the response body verbatim.
func WriteBody(w io.Writer, body []byte) error {
	_, err := w.Write(body)
	return err
}
