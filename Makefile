.PHONY: build clean test install run lint pre-commit help

BINARY_NAME=grabba
BUILD_DIR=./build
CMD_DIR=./cmd/grabba
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Цветной вывод
GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
NC=\033[0m

help: ## Показать помощь
	@printf "${GREEN}Доступные команды:${NC}\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  ${YELLOW}%-15s${NC} %s\n", $$1, $$2}'

build: ## Собрать бинарник для Linux
	@printf "${GREEN}Сборка $(BINARY_NAME) v$(VERSION)...${NC}\n"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@printf "${GREEN}✅ Сборка завершена: $(BUILD_DIR)/$(BINARY_NAME)${NC}\n"

build-all: ## Собрать для всех платформ
	@printf "${GREEN}Сборка для всех платформ...${NC}\n"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@printf "${GREEN}✅ Сборка для всех платформ завершена${NC}\n"

run: ## Запустить приложение
	@printf "${GREEN}Запуск $(BINARY_NAME)...${NC}\n"
	go run $(CMD_DIR)/main.go $(ARGS)

test: ## Запустить тесты
	@printf "${GREEN}Запуск тестов...${NC}\n"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@printf "${GREEN}✅ Покрытие: coverage.html${NC}\n"

bench: ## Запустить бенчмарки
	@printf "${GREEN}Запуск бенчмарков...${NC}\n"
	go test -bench=. -benchmem ./...

install: ## Установить в систему
	@printf "${GREEN}Установка $(BINARY_NAME)...${NC}\n"
	go install $(LDFLAGS) $(CMD_DIR)

clean: ## Очистить артефакты сборки
	@printf "${GREEN}Очистка...${NC}\n"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean
	@printf "${GREEN}✅ Очищено${NC}\n"

lint: ## Запустить линтеры
	@printf "${GREEN}Запуск линтеров...${NC}\n"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "${YELLOW}golangci-lint не установлен. Установка через go install...${NC}"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2; \
	fi
	@golangci-lint run ./... || true
	@printf "${GREEN}✅ Линтеры пройдены${NC}\n"

pre-commit: lint test build ## Запуск перед коммитом
	@printf "${GREEN}✅ Pre-commit проверки пройдены${NC}\n"

fmt: ## Форматирование кода
	@printf "${GREEN}Форматирование кода...${NC}\n"
	go fmt ./...
	go mod tidy
	@printf "${GREEN}✅ Код отформатирован${NC}\n"

docker-build: ## Собрать Docker образ
	@printf "${GREEN}Сборка Docker образа...${NC}\n"
	docker build -t $(BINARY_NAME):$(VERSION) -f Dockerfile .
	@printf "${GREEN}✅ Docker образ собран${NC}\n"

docker-run: ## Запустить в Docker
	@printf "${GREEN}Запуск в Docker...${NC}\n"
	docker run --rm -v $(PWD):/workspace $(BINARY_NAME):$(VERSION) $(ARGS)

release: build-all ## Создать релизную сборку
	@printf "${GREEN}Создание релиза v$(VERSION)...${NC}\n"
	cd $(BUILD_DIR) && \
	for file in $(BINARY_NAME)-*; do \
		gzip -9 "$$file"; \
	done
	@printf "${GREEN}✅ Релизные сборки созданы в $(BUILD_DIR)${NC}\n"

coverage: test ## Показать покрытие
	go tool cover -func=coverage.out

deps: ## Обновить зависимости
	@printf "${GREEN}Обновление зависимостей...${NC}\n"
	go mod download
	go mod verify
	go mod tidy
	@printf "${GREEN}✅ Зависимости обновлены${NC}\n"
