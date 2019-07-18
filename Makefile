ROOT_DIR := $(CURDIR)
BUILD_DIR := $(ROOT_DIR)/_output
BIN_DIR := $(BUILD_DIR)/bin
REGISTRY ?= "registry.cn-beijing.aliyuncs.com/yunionio"
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

LDFLAGS := "-w -s -X 'yunion.io/x/onecloud-operator/pkg/version.Version=${VERSION}'"

GOOS := $(if $(GOOS),$(GOOS),linux)
GOARCH := $(if $(GOARCH),$(GOARCH),amd64)
GOENV := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO := $(GOENV) go build

build: controller-manager

controller-manager:
	$(GO) -ldflags $(LDFLAGS) -o $(BIN_DIR)/onecloud-controller-manager cmd/controller-manager/main.go

image: build
	docker build -f images/onecloud-operator/Dockerfile -t $(REGISTRY)/onecloud-operator:$(VERSION) .
