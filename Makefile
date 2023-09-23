# Makefile with the following targets:
#   all: build the project
#   clean: remove all build artifacts
#   build: build the project
#   help: print this help message
#   .PHONY: mark targets as phony
#   .DEFAULT_GOAL: set the default goal to all

# Set the default goal to all
.DEFAULT_GOAL := all
PROJECT_NAME := "gh-token"

# Mark targets as phony
.PHONY: all clean build

# Build the project
all: clean build

# Remove all build artifacts
clean:
	rm gh-token

# Build the project
build:
	go build -o gh-token .
