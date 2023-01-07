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

TELEGRAF_INIT_VERSION=release-1.19.2-0

telegraf-init-image: telegraf-init
	DOCKER_DIR=${CURDIR}/images/telegraf-init IMAGE_KEYWORD=telegraf-init PUSH=true DEBUG=${DEBUG} REGISTRY=${REGISTRY} TAG=${TELEGRAF_INIT_VERSION} ARCH=all ${CURDIR}/scripts/docker_push.sh telegraf-init

fmt:
	@find . -type f -name "*.go" -not -path "./_output/*" \
		-not -path "./vendor/*" | xargs gofmt -s -w

fmt-check: fmt
	@if git status --short | grep -E '^.M .*/[^.]+.go'; then \
		git diff | cat; \
		echo "$@: working tree modified (possibly by gofmt)" >&2 ; \
		false ; \
	fi
.PHONY: fmt-check

goimports-check:
	@goimports -w -local "yunion.io/x/:yunion.io/x/onecloud:yunion.io/x/onecloud-operator" pkg cmd; \
    if git status --short | grep -E '^.M .*/[^.]+.go'; then \
        git diff | cat; \
        echo "$@: working tree modified (possibly by goimports)" >&2 ; \
        echo "$@: " >&2 ; \
        echo "$@: import spec should be grouped in order: std, 3rd-party, yunion.io/x, yunion.io/x/onecloud, yunion.io/x/onecloud-operator" >&2 ; \
        echo "$@: see \"yun\" branch at https://github.com/yousong/tools" >&2 ; \
        false ; \
    fi
.PHONY: goimports-check

check: fmt-check
check: goimports-check
.PHONY: check

RELEASE_BRANCH:=master
mod:
	GOPROXY=direct GOSUMDB=off go get yunion.io/x/onecloud@$(RELEASE_BRANCH)
	#go get $(patsubst %,%@master,$(shell GO111MODULE=on go mod edit -print | sed -n -e 's|.*\(yunion.io/x/[a-z].*\) v.*|\1|p' | grep -v '/onecloud$$'))
	go mod tidy
	go mod vendor -v

.PHONY: image telegraf-init-image mod fmt telegraf-init build onecloud-operator
