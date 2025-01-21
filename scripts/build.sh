#!/bin/bash
set -e

# 色の定義
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# ビルド設定
APP_NAME="face-emotion-analyzer"
BUILD_DIR="build"
BINARY_NAME="${APP_NAME}"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date +%FT%T%z)

# OSの検出
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# アーキテクチャの変換
case ${ARCH} in
    x86_64)
        GOARCH=amd64
        ;;
    arm64)
        GOARCH=arm64
        ;;
    *)
        GOARCH=${ARCH}
        ;;
esac

# ビルドフラグ
LDFLAGS="-X main.Version=${VERSION} \
         -X main.CommitHash=${COMMIT_HASH} \
         -X main.BuildTime=${BUILD_TIME} \
         -w -s"

echo -e "${GREEN}ビルドを開始します...${NC}"
echo "ターゲットプラットフォーム: ${OS}/${GOARCH}"

# ビルドディレクトリの作成
mkdir -p ${BUILD_DIR}

# 依存関係の検証
echo "依存関係を検証中..."
go mod verify
if [ $? -ne 0 ]; then
    echo -e "${RED}依存関係の検証に失敗しました${NC}"
    exit 1
fi

# 必要なパッケージの取得
echo "依存パッケージを取得中..."
go mod download
if [ $? -ne 0 ]; then
    echo -e "${RED}パッケージの取得に失敗しました${NC}"
    exit 1
fi

# ビルド前の静的解析
echo "静的解析を実行中..."
go vet ./...
if [ $? -ne 0 ]; then
    echo -e "${RED}静的解析でエラーが検出されました${NC}"
    exit 1
fi

# 本番用バイナリのビルド
echo "本番用バイナリをビルド中..."
CGO_ENABLED=1 GOOS=${OS} GOARCH=${GOARCH} \
go build -ldflags "${LDFLAGS}" \
         -o ${BUILD_DIR}/${BINARY_NAME} \
         cmd/server/main.go

if [ $? -ne 0 ]; then
    echo -e "${RED}ビルドに失敗しました${NC}"
    exit 1
fi

# 実行権限の設定
chmod +x ${BUILD_DIR}/${BINARY_NAME}

# 必要なファイルのコピー
echo "設定ファイルとモデルをコピー中..."
cp config/config.yaml ${BUILD_DIR}/
cp -r models ${BUILD_DIR}/
cp -r web ${BUILD_DIR}/

# 完了メッセージ
echo -e "${GREEN}ビルドが完了しました!${NC}"
echo "バイナリ: ${BUILD_DIR}/${BINARY_NAME}"
echo "バージョン: ${VERSION}"
echo "コミットハッシュ: ${COMMIT_HASH}"
echo "ビルド時刻: ${BUILD_TIME}"

# バイナリのサイズを表示
echo -e "\nバイナリ情報:"
ls -lh ${BUILD_DIR}/${BINARY_NAME}

# macOS以外の場合のみlddを実行
if [ "${OS}" != "darwin" ]; then
    echo -e "\n依存ライブラリ:"
    ldd ${BUILD_DIR}/${BINARY_NAME}
fi

# ビルドの検証
echo -e "\nビルドの検証中..."
${BUILD_DIR}/${BINARY_NAME} -version
if [ $? -ne 0 ]; then
    echo -e "${RED}ビルドの検証に失敗しました${NC}"
    exit 1
fi

echo -e "${GREEN}全ての処理が完了しました${NC}"