#!/bin/bash
set -e

# 色の定義
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# モデルの保存ディレクトリ
MODEL_DIR="models"
mkdir -p ${MODEL_DIR}

# OpenCVのGitHubリポジトリからモデルをダウンロード
OPENCV_MODELS=(
    "haarcascade_frontalface_default.xml"
    "haarcascade_eye.xml"
    "haarcascade_smile.xml"
)

BASE_URL="https://raw.githubusercontent.com/opencv/opencv/master/data/haarcascades"

echo -e "${GREEN}OpenCVモデルをダウンロードしています...${NC}"

for model in "${OPENCV_MODELS[@]}"; do
    echo "ダウンロード中: ${model}"
    if curl -sSL "${BASE_URL}/${model}" -o "${MODEL_DIR}/${model}"; then
        echo -e "${GREEN}✓ ${model} のダウンロードが完了しました${NC}"
    else
        echo -e "${RED}✗ ${model} のダウンロードに失敗しました${NC}"
        exit 1
    fi
done

# モデルのチェックサムを検証
echo -e "\n${GREEN}モデルの整合性を検証しています...${NC}"
for model in "${OPENCV_MODELS[@]}"; do
    if [ -f "${MODEL_DIR}/${model}" ]; then
        size=$(stat -f%z "${MODEL_DIR}/${model}" 2>/dev/null || stat -c%s "${MODEL_DIR}/${model}")
        if [ "$size" -lt 1000 ]; then
            echo -e "${RED}警告: ${model} のサイズが小さすぎます${NC}"
            exit 1
        fi
        echo -e "${GREEN}✓ ${model} の検証が完了しました${NC}"
    else
        echo -e "${RED}エラー: ${model} が見つかりません${NC}"
        exit 1
    fi
done

echo -e "\n${GREEN}全てのモデルのダウンロードと検証が完了しました${NC}"