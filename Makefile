.PHONY: build run test fmt lint clean

# Binary name
BINARY=ldap-manager
BINARY_PATH=./$(BINARY)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the application
build:
	@echo "Building $(BINARY)..."
	$(GOBUILD) -o $(BINARY_PATH) cmd/ldap-manager/main.go
	@echo "Build complete: $(BINARY_PATH)"

# Run the application
run:
	@echo "Running $(BINARY)..."
	$(GORUN) cmd/ldap-manager/main.go -config configs/config.yaml

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_PATH)
	@echo "Clean complete"

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) ./...
	$(GOMOD) download

# Run with race detector
run-race:
	@echo "Running with race detector..."
	$(GORUN) -race cmd/ldap-manager/main.go -config configs/config.yaml

# Build for production
build-prod:
	@echo "Building for production..."
	CGO_ENABLED=0 $(GOBUILD) -ldflags="-w -s" -o $(BINARY_PATH) cmd/ldap-manager/main.go
	@echo "Production build complete: $(BINARY_PATH)"

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  run        - Run the application"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  tidy       - Tidy dependencies"
	@echo "  clean      - Remove build artifacts"
	@echo "  deps       - Download dependencies"
	@echo "  run-race   - Run with race detector"
	@echo "  build-prod - Build optimized production binary"
