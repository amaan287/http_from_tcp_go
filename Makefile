build:
	@go build -o ./bin/httpfromtcp ./cmd/tcplistener/

run: build
	@./bin/httpfromtcp

test:
	@go test ./...