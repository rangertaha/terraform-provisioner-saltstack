# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
PLUGIN_DIR=.terraform/plugins
BINARY_NAME=terraform-provisioner-saltstack
VERSION=v0.1.0

.PHONY: help all

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'


all: build hash ## Build the binaries and output the md5 hash

hash: ## Output the md5 hash
	$(md5 -qs $(PLUGIN_DIR)/darwin_amd64/$(BINARY_NAME)_$(VERSION) || md5sum $(PLUGIN_DIR)/darwin_amd64/$(BINARY_NAME)_$(VERSION))
	$(md5 -qs $(PLUGIN_DIR)/linux_amd64/$(BINARY_NAME)_$(VERSION) || md5sum $(PLUGIN_DIR)/linux_amd64/$(BINARY_NAME)_$(VERSION))
	$(md5 -qs $(PLUGIN_DIR)/windows_amd64/$(BINARY_NAME)_$(VERSION) || md5sum $(PLUGIN_DIR)/windows_amd64/$(BINARY_NAME)_$(VERSION))

test: ## Run unit test
	$(GOTEST) -v ./...

clean: ## Remove files created by the build
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

deps: ## Install dependencies
	$(GOGET) github.com/rangertaha/terraform-provisioner-saltstack

build: clean deps test ## Build the binaries for Windows, OSX, and Linux
	$(mkdir -p $(PLUGIN_DIR))
	env GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(PLUGIN_DIR)/darwin_amd64/$(BINARY_NAME)_$(VERSION) -v
	env GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(PLUGIN_DIR)/linux_amd64/$(BINARY_NAME)_$(VERSION) -v
	env GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(PLUGIN_DIR)/windows_amd64/$(BINARY_NAME)_$(VERSION).exe -v





