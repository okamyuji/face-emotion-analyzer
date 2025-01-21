#!/bin/bash
set -e

# 色の定義
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}リントチェックを開始します...${NC}"

# 必要なツールの確認
REQUIRED_TOOLS=("golangci-lint" "goimports" "go" "staticcheck")

for tool in "${REQUIRED_TOOLS[@]}"; do
    if ! command -v "$tool" &> /dev/null; then
        echo -e "${RED}エラー: $tool がインストールされていません${NC}"
        exit 1
    fi
done

# goimportsによるコードフォーマット
echo "コードフォーマットをチェック中..."
if ! goimports -l -w .; then
    echo -e "${RED}goimportsでエラーが発生しました${NC}"
    exit 1
fi

# go vetによる基本的な静的解析
echo "go vetを実行中..."
if ! go vet ./...; then
    echo -e "${RED}go vetでエラーが発生しました${NC}"
    exit 1
fi

# staticcheckによる高度な静的解析
echo "staticcheckを実行中..."
if ! staticcheck ./...; then
    echo -e "${RED}staticcheckでエラーが発生しました${NC}"
    exit 1
fi

# golangci-lintによる包括的なリント
echo "golangci-lintを実行中..."
if ! golangci-lint run; then
    echo -e "${RED}golangci-lintでエラーが発生しました${NC}"
    exit 1
fi

# 循環的依存関係のチェック
echo "循環的依存関係をチェック中..."
if ! go mod graph | grep -v '@' | tsort >/dev/null 2>&1; then
    echo -e "${RED}循環的依存関係が検出されました${NC}"
    exit 1
fi

# 未使用のパッケージのチェック
echo "未使用のパッケージをチェック中..."
if ! go mod tidy -v; then
    echo -e "${RED}未使用のパッケージが検出されました${NC}"
    exit 1
fi

echo -e "${GREEN}全てのリントチェックが完了しました!${NC}"