PLUGIN_ID := cpa-account-vault
REGISTRY_FILE := $(CURDIR)/registry.json
GO ?= go
HOST_GO_RUN := env -u GOOS -u GOARCH $(GO) run
VERSION := $(shell $(HOST_GO_RUN) ./tools/version -registry "$(REGISTRY_FILE)" -plugin "$(PLUGIN_ID)")
DIST_DIR := $(CURDIR)/dist
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

ifeq ($(GOOS),darwin)
PLUGIN_EXT := dylib
else ifeq ($(GOOS),windows)
PLUGIN_EXT := dll
else
PLUGIN_EXT := so
endif

PLUGIN_FILE := $(DIST_DIR)/$(PLUGIN_ID).$(PLUGIN_EXT)
PACKAGE_FILE := $(DIST_DIR)/$(PLUGIN_ID)-$(VERSION)-$(GOOS)-$(GOARCH).zip

.PHONY: build plugin package test vet verify clean version check-version

build: plugin

plugin: check-version
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -buildvcs=false -ldflags "-X main.pluginVersion=$(VERSION)" -buildmode=c-shared -o $(PLUGIN_FILE) .
	rm -f $(DIST_DIR)/$(PLUGIN_ID).h

version: check-version
	@printf '%s\n' $(VERSION)

check-version:
	@$(HOST_GO_RUN) ./tools/version -registry "$(REGISTRY_FILE)" -plugin "$(PLUGIN_ID)" >/dev/null

package: plugin
	@command -v zip >/dev/null || (echo "zip is required for package" && exit 1)
	zip -j -q $(PACKAGE_FILE) $(PLUGIN_FILE)

test:
	$(GO) test . ./web

vet:
	$(GO) vet . ./web

verify:
	test -z "$$(gofmt -l $$(find . -name '*.go'))"
	$(GO) test . ./web
	$(MAKE) vet
	$(MAKE) build

clean:
	rm -rf $(DIST_DIR)
