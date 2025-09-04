# Makefile with the following targets:
#   all: build the project
#   clean: remove all build artifacts
#   build: build the project
#   test: run all unit tests
#   lint: run linting checks (golangci-lint)
#   install-lint-deps: install linting dependencies
#   release: create a new release by updating version numbers and committing changes
#   help: print this help message
#   .PHONY: mark targets as phony
#   .DEFAULT_GOAL: set the default goal to all

# Set the default goal to all
.DEFAULT_GOAL := all
PROJECT_NAME := "gh-token"

# Mark targets as phony
.PHONY: all clean build test lint install-lint-deps release

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

# Create a new release
release:
	@echo "Current version in main.go: $$(grep 'Version:' main.go | sed 's/.*Version: *"\(.*\)".*/\1/')"
	@echo "Current version in SECURITY.md: $$(grep -A2 '| Version' SECURITY.md | tail -1 | sed 's/| *\([0-9]*\.[0-9]*\.[0-9]*\).*/\1/')"
	@echo ""
	@read -p "Enter the new semver version (e.g., 2.1.0): " VERSION; \
	if [ -z "$$VERSION" ]; then \
		echo "Error: Version cannot be empty"; \
		exit 1; \
	fi; \
	if ! echo "$$VERSION" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$$' > /dev/null; then \
		echo "Error: Version must be in semver format (e.g., 2.1.0)"; \
		exit 1; \
	fi; \
	MAJOR_MINOR=$$(echo "$$VERSION" | sed 's/\([0-9]*\.[0-9]*\)\.[0-9]*/\1/'); \
	echo "Updating version to $$VERSION..."; \
	sed -i.bak 's/Version: *"[^"]*"/Version:              "'"$$VERSION"'"/' main.go && rm main.go.bak; \
	sed -i.bak 's/| [0-9]*\.[0-9]*\.[0-9]* *|/| '"$$MAJOR_MINOR"'.x   |/' SECURITY.md && rm SECURITY.md.bak; \
	echo "Files updated successfully."; \
	echo ""; \
	echo "Staging and committing changes..."; \
	git add main.go SECURITY.md; \
	git commit -m "Update version to $$VERSION"; \
	echo ""; \
	echo "Changes committed successfully!"; \
	echo ""; \
	echo "Next steps:"; \
	echo "- Go to https://github.com/Link-/gh-token/releases/new to create a new release"; \
	echo "- Create a tag with the same version as the release ($$VERSION)"; \
	echo "- The binaries will automatically be uploaded as assets once the release has been created"
