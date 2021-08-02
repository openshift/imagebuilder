export GOPROXY=https://proxy.golang.org

GO := go
# test for go module support
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
export GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
export GO_TEST=GO111MODULE=on $(GO) test -mod=vendor
else
export GO_BUILD=$(GO) build
export GO_TEST=$(GO) test
endif

build:
	$(GO_BUILD) ./cmd/imagebuilder
.PHONY: build

test:
	$(GO_TEST) $(shell go list ./... | grep -v /vendor/)
.PHONY: test

test-conformance:
	chmod -R go-w ./dockerclient/testdata
	$(GO_TEST) -v -tags conformance -timeout 30m ./dockerclient
.PHONY: test-conformance
