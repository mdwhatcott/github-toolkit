#!/usr/bin/make -f

VERSION := $(shell git describe)

test:
	go fmt ./...
	go mod tidy
	go test -cover -timeout=1s -race ./...

install: test
	go install -ldflags="-X 'main.Version=$(VERSION)'" github.com/mdwhatcott/github-toolkit/cmd/...