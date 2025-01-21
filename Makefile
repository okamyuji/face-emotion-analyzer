.PHONY: all build test lint clean run docker-build docker-run dev help

# 変数定義
APP_NAME := face-emotion-analyzer
BUILD_DIR := build
DOCKER_IMAGE := face-emotion-analyzer
DOCKER_TAG := latest

# デフォルトターゲット
all: help

# ビルド
build:
	@echo "Building $(APP_NAME)..."
	@./scripts/build.sh

# テスト実行
test:
	@echo "Running tests..."
	@./scripts/test.sh

# リント実行
lint:
	@echo "Running linters..."
	@./scripts/lint.sh

# クリーンアップ
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache -modcache

# ローカル実行
run: build
	@echo "Starting $(APP_NAME)..."
	@$(BUILD_DIR)/$(APP_NAME)

# Dockerイメージビルド
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Dockerコンテナ実行
docker-run: docker-build
	@echo "Running Docker container..."
	@docker run -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

# 開発環境起動
dev:
	@echo "Starting development environment..."
	@docker-compose up --build

# 開発環境のログ表示
logs:
	@docker-compose logs -f

# 開発環境の停止
stop:
	@echo "Stopping development environment..."
	@docker-compose down

# 依存関係の更新
deps:
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod verify

# カバレッジレポート生成
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# ツールのインストール
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest

# モックの生成
generate:
	@echo "Generating mocks..."
	@go generate ./...

# ヘルプ表示
help:
	@echo "Available commands:"
	@echo "  make build          - Build the application"
	@echo "  make test           - Run tests"
	@echo "  make lint           - Run linters"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make run           - Run the application locally"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run    - Run Docker container"
	@echo "  make dev           - Start development environment"
	@echo "  make logs          - Show development logs"
	@echo "  make stop          - Stop development environment"
	@echo "  make deps          - Update dependencies"
	@echo "  make coverage      - Generate coverage report"
	@echo "  make install-tools - Install development tools"
	@echo "  make generate      - Generate mocks"