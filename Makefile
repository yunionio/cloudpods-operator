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

build: onecloud-operator

onecloud-operator:
	$(GO) -mod vendor -ldflags $(LDFLAGS) -o $(BIN_DIR)/onecloud-controller-manager cmd/onecloud-operator/main.go

telegraf-init:
	$(GO) -mod vendor -ldflags $(LDFLAGS) -o $(BIN_DIR)/telegraf-init cmd/telegraf-init/main.go

image:
	DOCKER_DIR=${CURDIR}/images/onecloud-operator IMAGE_KEYWORD=onecloud-operator PUSH=true DEBUG=${DEBUG} REGISTRY=${REGISTRY} TAG=${VERSION} ARCH=${ARCH} ${CURDIR}/scripts/docker_push.sh onecloud-operator

telegraf-init-image: telegraf-init
	DOCKER_DIR=${CURDIR}/images/telegraf-init IMAGE_KEYWORD=telegraf-init PUSH=true DEBUG=${DEBUG} REGISTRY=${REGISTRY} TAG=${VERSION} ARCH=${ARCH} ${CURDIR}/scripts/docker_push.sh telegraf-init

fmt:
	find . -type f -name "*.go" -not -path "./_output/*" \
		-not -path "./vendor/*" | xargs gofmt -s -w

RELEASE_BRANCH:=release/3.8
mod:
	GOPROCY=direct go get yunion.io/x/onecloud@$(RELEASE_BRANCH)
	GOPROCY=direct go get $(patsubst %,%@master,$(shell GO111MODULE=on go mod edit -print | sed -n -e 's|.*\(yunion.io/x/[a-z].*\) v.*|\1|p' | grep -v '/onecloud$$' | grep -v sqlchemy))
	GOPROCY=direct go get yunion.io/x/sqlchemy@v1.0.2
	go mod tidy
	go mod vendor -v

.PHONY: image telegraf-init-image mod fmt telegraf-init build onecloud-operator
