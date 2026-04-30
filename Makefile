GO ?= go
DIST_DIR ?= dist
GOCACHE ?= /tmp/nvimon-gocache
CGO_ENABLED ?= 0
GOAMD64 ?= v1
BUILD_MODE ?= exe

GOENV = env GOCACHE=$(GOCACHE) CGO_ENABLED=$(CGO_ENABLED) GOAMD64=$(GOAMD64)
PORTABLE_DIR = $(DIST_DIR)/portable
LOCAL_NVML_DIR = $(DIST_DIR)/local-nvml

.PHONY: test build test-portable build-portable test-native build-native clean-dist build-binaries

test:
	$(GOENV) $(GO) test ./...

build:
	$(MAKE) clean-dist
	$(MAKE) build-portable
	$(MAKE) build-native

clean-dist:
	rm -rf $(PORTABLE_DIR) $(LOCAL_NVML_DIR)
	rm -f $(DIST_DIR)/nvimon $(DIST_DIR)/nvimon-agent

test-portable:
	$(MAKE) test CGO_ENABLED=0 GOAMD64=v1

build-portable:
	mkdir -p $(PORTABLE_DIR)
	$(MAKE) build-binaries DIST_OUT=$(PORTABLE_DIR) CGO_ENABLED=0 GOAMD64=v1

test-native:
	$(MAKE) test CGO_ENABLED=1

build-native:
	mkdir -p $(LOCAL_NVML_DIR)
	$(MAKE) build-binaries DIST_OUT=$(LOCAL_NVML_DIR) CGO_ENABLED=1

build-binaries:
	$(GOENV) $(GO) build -buildmode=$(BUILD_MODE) -o $(DIST_OUT)/nvimon ./cmd/nvimon
	$(GOENV) $(GO) build -buildmode=$(BUILD_MODE) -o $(DIST_OUT)/nvimon-agent ./cmd/nvimon-agent
	cp config.example.yaml $(DIST_DIR)/nvimon.config.yaml
