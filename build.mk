SHELL := /bin/bash

GO := go

BUILDDIR := dist

# capture version information
GITSHA := $(shell git rev-parse --short HEAD)
VERSION := $(shell echo "version")


CTIMEVAR=-X $(PKG)/version.GitCommit=$(GITSHA) -X $(PKG)/version.Version=$(VERSION)
GO_LDFLAGS=-ldflags "-w $(CTIMEVAR)"
GO_LDFLAGS_STATIC=-ldflags "-w $(CTIMEVAR) -extldflags -static"

all: clean build check install

.PHONY: clean
clean: ## Clean up any binaries and  build artifacts
	@echo "+ $@"
	$(RM) $(NAME)
	$(RM) -r $(BUILDDIR)

.PHONY: build
build: $(NAME)

$(NAME): $(wildcard *.go) $(wildcard */*.go)
	@echo "+ $@"
	$(GO) build -tags "$(BUILDTAGS)" ${GO_LDFLAGS} -o $(NAME) .

.PHONY: static
static:
	@echo "+ $@"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
				-tags "$(BUILDTAGS),netgo,osusergo,static_build" ${GO_LDFLAGS_STATIC} \
				-o $(NAME) .

.PHONY: release
release: build-release calculate-checksums ## Creates release artifacts

.PHONY: build-release
build-release: *.go ## Builds release binaries
	@echo "+ $@"
	CGO_ENABLED=$(CGO_ENABLED) gox \
		-os="darwin freebsd linux solaris windows" \
		-arch="amd64 arm arm64 386" \
		-osarch="!darwin/arm !darwin/arm64" \
		-output="$(BUILDDIR)/$(NAME)-{{.OS}}-{{.Arch}}" \
		-tags "$(BUILDTAGS),netgo,osusergo,static_build" \
		$(GO_LDFLAGS_STATIC)

define checksum
md5sum $(1) > $(1).md5;
sha256sum $(1) > $(1).sha256;
endef

.PHONY: calculate-checksums
calculate-checksums: $(wildcard BUILDDIR)/* ## Calculates checksums for release artifacts
	$(RM) $(BUILDDIR)/*.md5 $(BUILDDIR)/*.sha256
	$(foreach bin,$(wildcard $(BUILDDIR)/*), $(call checksum,$(bin)))


.PHONY: fmt
fmt: ## Makes sure go source files are formatted in the canonical format
	@echo "+ $@"
	@if [[ ! -z "$(shell gofmt -l -s . |  grep -v vendor | tee /dev/stderr)" ]]; then \
		exit 1; \
	fi

.PHONY: lint
lint: ## Makes sure `golint` 
	@echo "+ $@"
	@if [[ ! -z "$(shell golint ./... |  grep -v vendor | tee /dev/stderr)" ]]; then \
		exit 1; \
	fi

.PHONY: staticcheck
staticcheck: ## Makes sure `staticcheck` passes
	@echo "+ $@"
	@if [[ ! -z "$(shell staticcheck $(shell $(GO) list ./... | grep -v vendor) | tee /dev/stderr)" ]]; then \
		exit 1; \
	fi

.PHONY: vet
vet: ## Makes sure `go vet` passes
	@echo "+ $@"
	@if [[ ! -z "$(shell $(GO) vet $(shell $(GO) list ./... | grep -v vendor) | tee /dev/stderr)" ]]; then \
		exit 1; \
	fi

.PHONY: gosec
gosec: ## Makes sure `gosec` passes
	@echo "+ $@"
	@if [[ ! -z "$(shell gosec -quiet -fmt golint -confidence medium -severity medium ./... | tee /dev/stderr)" ]]; then \
		exit 1; \
	fi

.PHONY: test
test: ## Runs `go test` and makes sure the tests pass
	@echo "+ $@"
	@$(GO) test -v -tags "$(BUILDTAGS) cgo" $(shell $(GO) list ./... | grep -v vendor)

.PHONY: check
check: test fmt lint staticcheck vet ## Runs test, fmt, lint, staticcheck, and vet

.PHONY: install
	@echo "+ $@"
	@$(GO) install

.PHONY: tag
tag: ## Creates a new git tag for the current version
	git tag -sa $(VERSION) -m "$(VERSION)"
	git push origin master $(VERSION)

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | cut -d ':' -f2- | sort |  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
