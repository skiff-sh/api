SHELL := /bin/bash

GOBIN ?= $$(go env GOPATH)/bin

.PHONY: *

all:
	buf lint
	rm -rf api/go/*.pb.go
	buf generate
