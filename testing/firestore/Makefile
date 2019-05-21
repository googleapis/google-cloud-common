# This makefile builds the cross-language tests for Firestore.
# It outputs each test as a separate .textproto file into the
# testdata directory of this repo.

# Prerequisites:
# - This repo is under GOPATH.
# - protoc is on the path.
# - protoc supports proto3 syntax.
# - The Go protoc plugin has been downloaded and installed.
# - The github.com/google/protobuf repo has been cloned locally (go get github.com/google/protobuf).
# - The github.com/googleapis/googleapis repo has been cloned locally (go get github.com/googleapis/googleapis).

PROTOC = protoc

PROTOC_GO_PLUGIN_DIR = $(GOPATH)/bin

# Dependent repos.
PROTOBUF_REPO = $(GOPATH)/src/github.com/google/protobuf
GOOGLEAPIS_REPO = $(GOPATH)/src/github.com/googleapis/googleapis

.PHONY: generate-tests sync-protos gen-protos generator fmt

generate-tests: sync-protos gen-protos generator
	rm testdata/*.json
	$(GOPATH)/bin/generate-firestore-tests -o testdata

sync-protos:
	cd $(PROTOBUF_REPO) && git pull
	cd $(GOOGLEAPIS_REPO) && git pull

gen-protos: sync-protos
	mkdir -p genproto
	PATH=$(PATH):$(PROTOC_GO_PLUGIN_DIR) \
		$(PROTOC) --go_out=plugins=grpc:genproto \
		-I proto -I $(PROTOBUF_REPO)/src -I $(GOOGLEAPIS_REPO) \
		proto/*.proto

generator:
	go install ./cmd/generate-firestore-tests

fmt:
	gofmt -s -d -l -w ./cmd/generate-firestore-tests \
		&& goimports -w ./cmd/generate-firestore-tests
