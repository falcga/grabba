.PHONY: build clean test install run lint pre-commit help

BINARY_NAME=grabba
BUILD_DIR=./build
CMD_DIR=./cmd/grabba
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
NC=\033[0m

help:
	@printf "${GREEN}Available commands:${NC}\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  ${YELLOW}%-15s${NC} %s\n", $$1, $$2}'

build:
	@printf "${GREEN}[ ] Building $(BINARY_NAME) v$(VERSION)...${NC}\n"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR) || { printf "${RED}[-] Build failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Build complete: $(BUILD_DIR)/$(BINARY_NAME)${NC}\n"

build-all:
	@printf "${GREEN}[ ] Building for all platforms...${NC}\n"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR) || { printf "${RED}[-] Linux amd64 build failed${NC}\n"; exit 1; }
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR) || { printf "${RED}[-] Linux arm64 build failed${NC}\n"; exit 1; }
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR) || { printf "${RED}[-] Darwin amd64 build failed${NC}\n"; exit 1; }
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR) || { printf "${RED}[-] Darwin arm64 build failed${NC}\n"; exit 1; }
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR) || { printf "${RED}[-] Windows amd64 build failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Build for all platforms complete${NC}\n"

run:
	@printf "${GREEN}[ ] Running $(BINARY_NAME)...${NC}\n"
	go run $(CMD_DIR)/main.go $(ARGS) || { printf "${RED}[-] Run failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Run complete${NC}\n"

test:
	@printf "${GREEN}[ ] Running tests...${NC}\n"
	go test -v -race -coverprofile=coverage.out ./... || { printf "${RED}[-] Tests failed${NC}\n"; exit 1; }
	go tool cover -html=coverage.out -o coverage.html
	@printf "${GREEN}[+] Coverage: coverage.html${NC}\n"

bench:
	@printf "${GREEN}[ ] Running benchmarks...${NC}\n"
	go test -bench=. -benchmem ./... || { printf "${RED}[-] Benchmarks failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Benchmarks complete${NC}\n"

install:
	@printf "${GREEN}[ ] Installing $(BINARY_NAME)...${NC}\n"
	go install $(LDFLAGS) $(CMD_DIR) || { printf "${RED}[-] Install failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Install complete${NC}\n"

clean:
	@printf "${GREEN}[ ] Cleaning...${NC}\n"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean
	@printf "${GREEN}[+] Cleaned${NC}\n"

lint:
	@printf "${GREEN}[ ] Running linters...${NC}\n"
	@command -v golangci-lint >/dev/null 2>&1 || { \
		printf "${YELLOW}[ ] golangci-lint not installed. Installing...${NC}\n"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2 || { printf "${RED}[-] Lint installation failed${NC}\n"; exit 1; }; \
	}
	golangci-lint run ./... || { printf "${RED}[-] Linters failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Linters passed${NC}\n"

pre-commit: lint test build
	@printf "${GREEN}[+] Pre-commit checks passed${NC}\n"

fmt:
	@printf "${GREEN}[ ] Formatting code...${NC}\n"
	go fmt ./... || { printf "${RED}[-] Format failed${NC}\n"; exit 1; }
	go mod tidy || { printf "${RED}[-] Mod tidy failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Code formatted${NC}\n"

docker-build:
	@printf "${GREEN}[ ] Building Docker image...${NC}\n"
	docker build -t $(BINARY_NAME):$(VERSION) -f Dockerfile . || { printf "${RED}[-] Docker build failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Docker image built${NC}\n"

docker-run:
	@printf "${GREEN}[ ] Running in Docker...${NC}\n"
	docker run --rm -v $(PWD):/workspace $(BINARY_NAME):$(VERSION) $(ARGS) || { printf "${RED}[-] Docker run failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Docker run complete${NC}\n"

release: build-all
	@printf "${GREEN}[ ] Creating release v$(VERSION)...${NC}\n"
	cd $(BUILD_DIR) && \
	for file in $(BINARY_NAME)-*; do \
		gzip -9 "$$file" || { printf "${RED}[-] Compression failed for $$file${NC}\n"; exit 1; }; \
	done
	@printf "${GREEN}[+] Release builds created in $(BUILD_DIR)${NC}\n"

coverage: test
	go tool cover -func=coverage.out

deps:
	@printf "${GREEN}[ ] Updating dependencies...${NC}\n"
	go mod download || { printf "${RED}[-] Download failed${NC}\n"; exit 1; }
	go mod verify || { printf "${RED}[-] Verify failed${NC}\n"; exit 1; }
	go mod tidy || { printf "${RED}[-] Tidy failed${NC}\n"; exit 1; }
	@printf "${GREEN}[+] Dependencies updated${NC}\n"
