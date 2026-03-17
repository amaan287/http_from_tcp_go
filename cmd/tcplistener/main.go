package main

import (
	"fmt"
	"log"
	"net"

	"github.com/amaan287/httpserver/internal/request"
)

func main() {
	listener, err := net.Listen("tcp", ":42069")
	if err != nil {
		log.Fatalf("error reading opening file: %v", err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		r, err := request.RequestFromReader(conn)
		if err != nil {
			log.Fatalf("error %v", err)
		}
		fmt.Printf("Request line: %s\n", r.RequestLine)
		fmt.Printf(" -Method: %s\n", r.RequestLine.Method)
		fmt.Printf(" -Target: %s\n", r.RequestLine.RequestTarget)
		fmt.Printf(" -Version: %s\n", r.RequestLine.HttpVersion)
	}
}
