.PHONY: build dev run test clean docker-build docker-up docker-down docker-logs install uninstall help

# Default target
help:
	@echo "Homeport - Development Commands"
	@echo ""
	@echo "Development:"
	@echo "  make dev          Run daemon in dev mode"
	@echo "  make build        Build binaries for current platform"
	@echo "  make build-linux  Cross-compile for Linux"
	@echo "  make ui           Build the UI"
	@echo "  make clean        Remove build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build Build Docker images"
	@echo "  make docker-up    Start Docker containers"
	@echo "  make docker-down  Stop Docker containers"
	@echo "  make docker-logs  View container logs"
	@echo ""
	@echo "Install:"
	@echo "  make install      Run install script"
	@echo "  make uninstall    Remove Homeport"

# === Development ===

dev:
	go run ./cmd/homeportd --dev

build: bin/homeportd bin/homeport

bin/homeportd: $(shell find . -name '*.go')
	@mkdir -p bin
	go build -o bin/homeportd ./cmd/homeportd

bin/homeport: $(shell find . -name '*.go')
	@mkdir -p bin
	go build -o bin/homeport ./cmd/homeport

build-linux:
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/homeportd-linux ./cmd/homeportd
	GOOS=linux GOARCH=amd64 go build -o bin/homeport-linux ./cmd/homeport

ui:
	cd ui && npm install && npm run build

clean:
	rm -rf bin/
	rm -rf ui/dist/
	rm -rf ui/node_modules/

# === Docker ===

docker-build:
	cd docker && docker compose build

docker-up:
	cd docker && docker compose up -d

docker-down:
	cd docker && docker compose down

docker-logs:
	cd docker && docker compose logs -f

docker-restart:
	cd docker && docker compose restart

# === Install ===

install:
	./scripts/install.sh

uninstall:
	./scripts/uninstall.sh
