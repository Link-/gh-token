# Makefile with the following targets:
#   all: build the project
#   clean: remove all build artifacts
#   build: build the project
#   test: run all unit tests
#   lint: run linting checks (golangci-lint)
#   install-lint-deps: install linting dependencies
#   help: print this help message
#   .PHONY: mark targets as phony
#   .DEFAULT_GOAL: set the default goal to all

# Set the default goal to all
.DEFAULT_GOAL := all
PROJECT_NAME := "gh-token"

# Mark targets as phony
.PHONY: all clean build test lint install-lint-deps

# Build the project
all: clean build

# Remove all build artifacts
clean:
	rm -f gh-token
	rm -rf .bin

# Build the project
build:
	go build -o gh-token .

# Run all unit tests
test:
	go test ./...

# Run linting checks
lint:
	@test -f .bin/golangci-lint || $(MAKE) install-lint-deps
	./.bin/golangci-lint run

# Install linting dependencies
install-lint-deps:
	@mkdir -p .bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b .bin v2.4.0
