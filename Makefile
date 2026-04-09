.PHONY: build test lint

build:
	go build -o bin/pgmigrator .

test:
	go test ./...

lint:
	golangci-lint run
