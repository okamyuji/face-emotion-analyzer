#!/bin/bash
set -e

# 色の定義
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}テストを開始します...${NC}"

# テスト環境の準備
export APP_ENV=test
export OPENCV_TEST_DATA="$(pwd)/models"

# レースコンディションのテストを有効化
export GORACE="halt_on_error=1"

# キャッシュの削除
go clean -testcache

# テストの実行
echo "標準テストを実行中..."
go test -race -v -cover ./...

# ベンチマークの実行
echo "ベンチマークテストを実行中..."
go test -bench=. -benchmem ./...

# テストカバレッジレポートの生成
echo "テストカバレッジレポートを生成中..."
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html

echo -e "${GREEN}テスト完了!${NC}"

# カバレッジの閾値チェック
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
THRESHOLD=80

if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo -e "${RED}カバレッジが閾値(${THRESHOLD}%)を下回っています: ${COVERAGE}%${NC}"
    exit 1
fi

echo -e "${GREEN}カバレッジ基準をクリアしました: ${COVERAGE}%${NC}"