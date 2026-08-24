package main

import (
	"fmt"
	"log"
	"net"

	"github.com/amaan287/httpserver/internal/request"
	"github.com/amaan287/httpserver/internal/response"
)

const listenAddr = ":42069"

func main() {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("error opening listener: %v", err)
	}
	defer listener.Close()

	log.Printf("listening on %s", listenAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("error accepting connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	r, err := request.RequestFromReader(conn)
	if err != nil {
		log.Printf("error parsing request: %v", err)
		writeResponse(conn, response.StatusBadRequest, nil)
		return
	}

	fmt.Printf("Request line: %+v\n", r.RequestLine)
	fmt.Printf(" -Method: %s\n", r.RequestLine.Method)
	fmt.Printf(" -Target: %s\n", r.RequestLine.RequestTarget)
	fmt.Printf(" -Version: %s\n", r.RequestLine.HttpVersion)
	r.Headers.ForEach(func(n, v string) {
		fmt.Printf("- %s: %s\n", n, v)
	})

	if err := writeResponse(conn, response.StatusOK, nil); err != nil {
		log.Printf("error writing response: %v", err)
	}
}

func writeResponse(w net.Conn, status response.StatusCode, body []byte) error {
	if err := response.WriteStatusLine(w, status); err != nil {
		return err
	}
	if err := response.WriteHeaders(w, response.GetDefaultHeaders(len(body))); err != nil {
		return err
	}
	return response.WriteBody(w, body)
}
