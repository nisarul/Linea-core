.PHONY: all build test test-race cover lint vuln tidy clean

all: build test

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

cover: test-race
	go tool cover -func=coverage.out

lint:
	golangci-lint run

vuln:
	govulncheck ./...

tidy:
	go mod tidy

clean:
	rm -rf coverage.out coverage.html bin dist
