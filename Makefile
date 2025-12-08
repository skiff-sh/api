SHELL := /bin/bash

GOBIN ?= $$(go env GOPATH)/bin

.PHONY: *

all: build.plain-go
	buf lint
	etc/pre-gen.sh
	buf generate --exclude-path "proto/skiff/plugin"
	buf generate --template buf-plugin.gen.yaml --path "proto/skiff/plugin"
	etc/post-gen.sh

build.plain-go:
	cd protoc-gen-plain-go && go build -o protoc-gen-plain-go

croc.receive:
	croc --yes --overwrite skiff123
