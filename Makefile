.PHONY: build test lint add_test_case

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/pgmigrator .

test:
	go test ./...

lint:
	golangci-lint run

add_test_case:
	@if [ -z "$(URL)" ]; then \
		echo "Usage: make add_test_case URL=postgres://user:pass@host:5432/dbname"; \
		exit 1; \
	fi
	@name=$$(echo "$(URL)" | sed 's|.*://||; s|.*/||; s|\?.*||'); \
	outfile="testdata/schemas/$${name}.sql"; \
	pg_dump --schema-only --no-owner --no-privileges "$(URL)" > "$${outfile}" && \
	echo "Added $${outfile}"
