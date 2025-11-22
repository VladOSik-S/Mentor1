.PHONY: help install build-ui build-server build run dev clean test lint docker-build

# Переменные
BINARY_NAME=menti-hub
UI_DIR=_ui
BUILD_DIR=build
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")

# Цвета для вывода
CYAN=\033[0;36m
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

## help: Показать справку
help:
	@echo "$(CYAN)Доступные команды:$(NC)"
	@echo "  $(GREEN)make install$(NC)      - Установить все зависимости"
	@echo "  $(GREEN)make build$(NC)        - Собрать полный проект (UI + backend)"
	@echo "  $(GREEN)make build-ui$(NC)     - Собрать только React приложение"
	@echo "  $(GREEN)make build-server$(NC) - Собрать только Go бинарник"
	@echo "  $(GREEN)make run$(NC)          - Собрать и запустить сервер"
	@echo "  $(GREEN)make dev$(NC)          - Запустить в режиме разработки"
	@echo "  $(GREEN)make dev-ui$(NC)       - Запустить только React dev server"
	@echo "  $(GREEN)make clean$(NC)        - Очистить собранные файлы"
	@echo "  $(GREEN)make test$(NC)         - Запустить тесты"
	@echo "  $(GREEN)make lint$(NC)         - Запустить линтеры"
	@echo "  $(GREEN)make docker-build$(NC) - Собрать Docker образ"
	@echo "  $(GREEN)make swagger$(NC)      - Генерировать Swagger документацию"

## install: Установить зависимости для UI и Go
install:
	@echo "$(CYAN)📦 Установка зависимостей...$(NC)"
	@cd $(UI_DIR) && npm install
	@go mod download
	@echo "$(GREEN)✓ Зависимости установлены$(NC)"

## build-ui: Собрать React приложение
build-ui:
	@echo "$(CYAN)🎨 Сборка React приложения...$(NC)"
	@cd $(UI_DIR) && npm run build
	@echo "$(GREEN)✓ React приложение собрано$(NC)"

## build-server: Собрать Go бинарник
build-server:
	@echo "$(CYAN)🔨 Сборка Go сервера...$(NC)"
	@go build -o $(BINARY_NAME) -ldflags="-s -w" .
	@echo "$(GREEN)✓ Go бинарник собран: $(BINARY_NAME)$(NC)"

## build: Полная сборка проекта
build: build-ui build-server
	@echo "$(GREEN)✓ Проект полностью собран$(NC)"

## run: Собрать и запустить сервер
run: build
	@echo "$(CYAN)🚀 Запуск сервера...$(NC)"
	@./$(BINARY_NAME)

## dev: Запустить в режиме разработки (без сборки UI)
dev:
	@echo "$(CYAN)🔧 Запуск в режиме разработки...$(NC)"
	@go run .

## dev-ui: Запустить только React dev server
dev-ui:
	@echo "$(CYAN)🎨 Запуск React dev server...$(NC)"
	@cd $(UI_DIR) && npm start

## dev-all: Запустить React и Go сервер одновременно
dev-all:
	@echo "$(CYAN)🚀 Запуск полного dev окружения...$(NC)"
	@trap 'kill 0' INT; \
	(cd $(UI_DIR) && npm start) & \
	go run . & \
	wait

## clean: Очистить собранные файлы
clean:
	@echo "$(YELLOW)🧹 Очистка...$(NC)"
	@rm -f $(BINARY_NAME)
	@rm -rf $(UI_DIR)/build
	@rm -rf $(UI_DIR)/node_modules/.cache
	@echo "$(GREEN)✓ Очищено$(NC)"

## clean-all: Полная очистка (включая node_modules)
clean-all: clean
	@echo "$(YELLOW)🧹 Полная очистка...$(NC)"
	@rm -rf $(UI_DIR)/node_modules
	@go clean -cache
	@echo "$(GREEN)✓ Полностью очищено$(NC)"

## test: Запустить тесты
test:
	@echo "$(CYAN)🧪 Запуск тестов...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@cd $(UI_DIR) && npm test -- --watchAll=false

## test-coverage: Запустить тесты с покрытием
test-coverage: test
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Отчёт о покрытии: coverage.html$(NC)"

## lint: Запустить линтеры
lint:
	@echo "$(CYAN)🔍 Запуск линтеров...$(NC)"
	@golangci-lint run ./... || echo "$(YELLOW)Установите golangci-lint: https://golangci-lint.run/$(NC)"
	@cd $(UI_DIR) && npm run lint || echo "$(YELLOW)Добавьте lint скрипт в package.json$(NC)"

## fmt: Форматировать код
fmt:
	@echo "$(CYAN)✨ Форматирование кода...$(NC)"
	@go fmt ./...
	@cd $(UI_DIR) && npm run format || npx prettier --write "src/**/*.{js,jsx,ts,tsx,json,css,md}"
	@echo "$(GREEN)✓ Код отформатирован$(NC)"

## swagger: Генерировать Swagger документацию
swagger:
	@echo "$(CYAN)📚 Генерация Swagger документации...$(NC)"
	@swag init || echo "$(YELLOW)Установите swag: go install github.com/swaggo/swag/cmd/swag@latest$(NC)"

## docker-build: Собрать Docker образ
docker-build:
	@echo "$(CYAN)🐳 Сборка Docker образа...$(NC)"
	@docker build -t $(BINARY_NAME):latest .
	@echo "$(GREEN)✓ Docker образ собран$(NC)"

## docker-run: Запустить Docker контейнер
docker-run:
	@echo "$(CYAN)🐳 Запуск Docker контейнера...$(NC)"
	@docker run -p 8080:8080 $(BINARY_NAME):latest

## watch: Автоперезапуск при изменениях (требует air)
watch:
	@echo "$(CYAN)👀 Запуск с автоперезапуском...$(NC)"
	@air || echo "$(YELLOW)Установите air: go install github.com/air-verse/air@latest$(NC)"

## deps-upgrade: Обновить зависимости
deps-upgrade:
	@echo "$(CYAN)📦 Обновление зависимостей...$(NC)"
	@go get -u ./...
	@go mod tidy
	@cd $(UI_DIR) && npm update
	@echo "$(GREEN)✓ Зависимости обновлены$(NC)"

## check: Проверить проект перед коммитом
check: fmt lint test
	@echo "$(GREEN)✓ Все проверки пройдены$(NC)"

# Версионирование
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

## build-release: Сборка релизной версии с версией
build-release: build-ui
	@echo "$(CYAN)📦 Сборка релиза версии $(VERSION)...$(NC)"
	@go build -o $(BINARY_NAME) \
		-ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" .
	@echo "$(GREEN)✓ Релиз собран: $(BINARY_NAME) v$(VERSION)$(NC)"

## info: Показать информацию о проекте
info:
	@echo "$(CYAN)Информация о проекте:$(NC)"
	@echo "  Бинарник: $(BINARY_NAME)"
	@echo "  Go версия: $(shell go version)"
	@echo "  Node версия: $(shell cd $(UI_DIR) && node --version)"
	@echo "  Текущая ветка: $(shell git branch --show-current)"
	@echo "  Последний коммит: $(shell git log -1 --pretty=format:'%h - %s')"

