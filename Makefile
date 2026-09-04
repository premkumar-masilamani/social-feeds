.PHONY: all build run test lint fmt clean help

APP_NAME := social-rss
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(APP_NAME)
MAIN_PKG := ./cmd/social-rss

all: build

## build: Build the Go binary
build:
	@echo "==> Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) $(MAIN_PKG)
	@echo "==> Build complete: $(BINARY)"

## run: Build and run the application
run: build
	@echo "==> Running $(APP_NAME)..."
	./$(BINARY)

## test: Run unit tests with race detection
test:
	@echo "==> Running tests..."
	go test -v -race ./...

## lint: Run go vet and check code formatting
lint:
	@echo "==> Running go vet..."
	go vet ./...
	@echo "==> Checking gofmt formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found. Run 'make fmt':" && gofmt -l . && exit 1)
	@echo "==> Linting passed successfully."

## fmt: Automatically format all Go code
fmt:
	@echo "==> Formatting code with gofmt..."
	gofmt -s -w .

## clean: Remove build artifacts and temporary files
clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out

## help: Display available make targets
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':'
