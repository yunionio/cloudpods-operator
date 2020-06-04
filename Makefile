export GO111MODULE:=on
export GOPROXY:=direct

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
	$(GO) -mod vendor -ldflags $(LDFLAGS) -o $(BIN_DIR)/onecloud-controller-manager cmd/controller-manager/main.go

image: build
	sudo docker build -f images/onecloud-operator/Dockerfile -t $(REGISTRY)/onecloud-operator:$(VERSION) .
	sudo docker push $(REGISTRY)/onecloud-operator:$(VERSION)

fmt:
	find . -type f -name "*.go" -not -path "./_output/*" \
		-not -path "./vendor/*" | xargs gofmt -s -w

RELEASE_BRANCH:=release/3.2
mod:
	go get yunion.io/x/onecloud@$(RELEASE_BRANCH)
	go get $(patsubst %,%@master,$(shell GO111MODULE=on go mod edit -print | sed -n -e 's|.*\(yunion.io/x/[a-z].*\) v.*|\1|p' | grep -v '/onecloud$$'))
	go mod tidy
	go mod vendor -v
