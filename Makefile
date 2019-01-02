.PHONY: test deps
.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on
LOCALS      := $(shell find . -type f -name '*.go')

all: fmt deps build

deps:
	go generate -x
	go get .

fmt:
	gofmt -w .
	go vet .

test:
	go test -race .

build: fmt
	go build -i -o bin/moped .