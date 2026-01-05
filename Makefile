SHELL := /bin/bash

GOBIN ?= $$(go env GOPATH)/bin

.PHONY: *

all: build.plain-go
	buf lint
	etc/pre-gen.sh
	buf generate --exclude-path "proto/skiff/plugin"
	buf generate --template buf-plugin.gen.yaml --path "proto/skiff/plugin"
	cd etc/postgen && go build
	./etc/postgen/postgen

build.plain-go:
	cd protoc-gen-plain-go && go build -o protoc-gen-plain-go
