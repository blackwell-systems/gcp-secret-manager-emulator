.PHONY: help build build-grpc build-rest build-dual install install-grpc install-rest install-dual test clean docker docker-grpc docker-rest docker-dual

# Default target
help:
	@echo "GCP Secret Manager Emulator - Build Targets"
	@echo ""
	@echo "Build commands:"
	@echo "  make build          - Build all server variants"
	@echo "  make build-grpc     - Build gRPC-only server (default)"
	@echo "  make build-rest     - Build REST-only server"
	@echo "  make build-dual     - Build dual-protocol server"
	@echo ""
	@echo "Install commands:"
	@echo "  make install        - Install all server variants to GOPATH/bin"
	@echo "  make install-grpc   - Install gRPC-only server"
	@echo "  make install-rest   - Install REST-only server"
	@echo "  make install-dual   - Install dual-protocol server"
	@echo ""
	@echo "Docker commands:"
	@echo "  make docker         - Build all Docker variants"
	@echo "  make docker-grpc    - Build gRPC-only Docker image"
	@echo "  make docker-rest    - Build REST-only Docker image"
	@echo "  make docker-dual    - Build dual-protocol Docker image"
	@echo ""
	@echo "Test commands:"
	@echo "  make test           - Run all tests"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo ""
	@echo "Other commands:"
	@echo "  make clean          - Remove built binaries"

# Build all variants
build: build-grpc build-rest build-dual

# Build gRPC-only server
build-grpc:
	@echo "Building gRPC-only server..."
	go build -o bin/server ./cmd/server

# Build REST-only server
build-rest:
	@echo "Building REST-only server..."
	go build -o bin/server-rest ./cmd/server-rest

# Build dual-protocol server
build-dual:
	@echo "Building dual-protocol server..."
	go build -o bin/server-dual ./cmd/server-dual

# Install all variants
install: install-grpc install-rest install-dual

# Install gRPC-only server
install-grpc:
	@echo "Installing gRPC-only server..."
	go install ./cmd/server

# Install REST-only server
install-rest:
	@echo "Installing REST-only server..."
	go install ./cmd/server-rest

# Install dual-protocol server
install-dual:
	@echo "Installing dual-protocol server..."
	go install ./cmd/server-dual

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean built binaries
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run gRPC server locally
run-grpc: build-grpc
	./bin/server

# Run REST server locally
run-rest: build-rest
	./bin/server-rest

# Run dual-protocol server locally
run-dual: build-dual
	./bin/server-dual

# Docker build targets
docker: docker-grpc docker-rest docker-dual

docker-grpc:
	@echo "Building gRPC-only Docker image..."
	docker build --build-arg VARIANT=grpc -t gcp-secret-manager-emulator:grpc -t gcp-secret-manager-emulator:latest .

docker-rest:
	@echo "Building REST-only Docker image..."
	docker build --build-arg VARIANT=rest -t gcp-secret-manager-emulator:rest .

docker-dual:
	@echo "Building dual-protocol Docker image..."
	docker build --build-arg VARIANT=dual -t gcp-secret-manager-emulator:dual .
