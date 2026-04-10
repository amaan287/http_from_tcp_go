build:
	@go build -o ./cmd/tcplistner/main.go ./bin/httpfromtcp

run: build
	@./bin/httpfromtcp

test:
	@go test ./...