SHELL := bash
.SHELLFLAGS := -e -o pipefail -c
MAKEFLAGS   += --warn-undefined-variables

ifeq ($(GOPATH),)
	GOPATH = $(HOME)/go
endif

GOBIN               ?= $(GOPATH)/bin
GOOS                ?= $(shell go env GOOS)
GOARCH              ?= $(shell go env GOARCH)
container_test_args := -test.v $(TEST_ARGS)

ifdef SUDO_USER
	SKIP_ADMIN_TEST = 0
endif

export SKIP_ADMIN_TEST
export GOPATH
export GOBIN

# build time variables embedded in final executable
VERSION    ?= "1.2.0"
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%S)
COMMIT_ID  := $(strip $(shell git rev-parse HEAD))
# strip debug info from binaries
GO_BUILD_LDFLAGS := -s -w
# set build time variables
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.Version=$(VERSION)
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.BuildTime=$(BUILD_TIME)
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.CommitID=$(COMMIT_ID)

GO_BUILD_TAGS = netgo osusergo
ifeq ($(GOOS),darwin)
	CGO_CFLAGS = -mmacosx-version-min=10.15
	CGO_LDFLAGS = -mmacosx-version-min=10.15
    # export SDKROOT=$(shell xcrun --sdk macosx --show-sdk-path)
    export CGO_CFLAGS
    export CGO_LDFLAGS
endif

ifdef disable_test
	disable_test_race = 1
endif

go_test := CGO_ENABLED=0 go test -v -count=1 -failfast \
		-ldflags "$(GO_BUILD_LDFLAGS)" -tags "$(GO_BUILD_TAGS)"

version_output_file           ?= ./passwall-server_version.json

.PHONY: all
all: test build

.PHONY: test
test: get generate lint
ifndef disable_test
    # internal tests
	$(go_test) -coverpkg=./... -coverprofile=$(GOOS)-cover.out ./...
	$(GOBIN)/gocov convert $(GOOS)-cover.out | $(GOBIN)/gocov report
endif
    # -race is not supported on 386 architecture
    # -race tests fail under docker arm64 container due to emulation.
ifndef disable_test_race
ifneq ($(GOARCH),386)
	$(go_test) -race -failfast ./...
endif
endif


.PHONY: lint
lint:
ifndef disable_lint
	$(GOBIN)/golangci-lint run --timeout 15m ./...
endif

.PHONY: get
get:
	go version
	go env
	@echo -------------
	@echo GOBIN=$(GOBIN)
	@echo GOPATH=$(GOPATH)
	mkdir -p $(GOBIN)
	mkdir -p $(GOPATH)
ifndef disable_lint
	# install golangci-lint, Note: always check version and update below.
	[ ! -f $(GOBIN)/golangci-lint ] && \
		CGO_ENABLED=0 curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(GOBIN) v1.44.2 || true
	[ -x $(GOBIN)/golangci-lint ]
endif
ifndef disable_test
	# install coverage reporter
	[ ! -f $(GOBIN)/gocov ] && \
		go install github.com/axw/gocov/gocov@latest || true
	[ -x $(GOBIN)/gocov ]
endif

.PHONY: build
build: generate
	go version
	go env
	rm -f ./passwall-server
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -tags "$(GO_BUILD_TAGS)" \
		-ldflags "$(GO_BUILD_LDFLAGS)" -o ./passwall-server ./cmd/passwall-server
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -tags "$(GO_BUILD_TAGS)" \
		-ldflags "$(GO_BUILD_LDFLAGS)" -o ./passwall-cli ./cmd/passwall-cli


# We embed files in pkg/embedded package with go generate command.
.PHONY: generate
generate:
	go generate ./...

.PHONY: clean
clean:
	rm -f *-cover.out
	rm -f gotest_*.test
	rm -f *.log
	rm -f ./passwall-server
	find . -iname "*.log" -type f -delete || true