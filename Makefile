BUILD_DIR := .build
SVC_NAME := lazypass
REPOSITORY := lazypass
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build run tidy fmt fmt-check test test-cover lint snapshot release

build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w -X $(REPOSITORY)/internal/build.Version=$(VERSION)" -o $(BUILD_DIR)/$(SVC_NAME) .

run: build
	./$(BUILD_DIR)/$(SVC_NAME)

tidy:
	go mod tidy

fmt:
	gofmt -w cmd internal main.go

fmt-check:
	test -z "$$(gofmt -l cmd internal main.go)"

test:
	go test -race ./...

test-cover:
	go test -race -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
