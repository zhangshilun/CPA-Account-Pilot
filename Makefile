PLUGIN_ID := cpa-account-pilot
VERSION ?= 0.1.0
GO ?= go
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

.PHONY: build plugin package test vet verify clean

build: plugin

plugin:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -buildvcs=false -ldflags "-X main.pluginVersion=$(VERSION)" -buildmode=c-shared -o $(PLUGIN_FILE) .
	rm -f $(DIST_DIR)/$(PLUGIN_ID).h

package: plugin
	@command -v zip >/dev/null || (echo "zip is required for package" && exit 1)
	zip -j -q $(PACKAGE_FILE) $(PLUGIN_FILE)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

verify:
	test -z "$$(gofmt -l $$(find . -name '*.go'))"
	$(GO) test ./...
	$(MAKE) vet
	$(MAKE) build

clean:
	rm -rf $(DIST_DIR)
