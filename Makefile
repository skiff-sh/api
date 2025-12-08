SHELL := /bin/bash

GOBIN ?= $$(go env GOPATH)/bin

.PHONY: *

all: build.plain-go
	buf lint
	find go -type f -name '*.go' -delete
	buf generate --exclude-path "proto/skiff/plugin"
	buf generate --template buf-plugin.gen.yaml --path "proto/skiff/plugin"

build.plain-go:
	cd protoc-gen-plain-go && go build -o protoc-gen-plain-go

