
# Image URL to use all building/pushing image targets
REGISTRY ?= docker.io
REPO ?= dynamic-watcher
TAG ?= $(shell git rev-parse --short HEAD)
IMG ?= ${REGISTRY}/${REPO}/dynamic-runner-webhook-watcher:${TAG}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
# What the controller will be referred to as
CONTROLLER_NAME ?= dynamic-webhook-watcher

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golangci-lint against code.
	golangci-lint run

test: fmt vet
	go test -v ./... -coverprofile cover.out

##@ Build

build: fmt vet ## Build manager binary.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-X 'main.Build=$(git rev-parse --short HEAD)'" \
		-o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go $(ARGS)

docker-build: test build
	docker build -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

validate-chart: kubeval
	@helm template test ./charts/runner-webhook-watcher| $(KUBEVAL)

KUBEVAL = $(shell pwd)/bin/kubeval
kubeval: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUBEVAL),github.com/instrumenta/kubeval@v0.16.1)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# installs the specified version of golangci-lint
# binary will be $(go env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION=v1.42.1
install-golangci-lint:
ifeq (, $(findstring $(GOLANGCI_LINT_VERSION),$(shell golangci-lint --version)))
	@{ \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION) ; \
	}
endif
