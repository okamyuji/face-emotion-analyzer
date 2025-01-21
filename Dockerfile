# ビルドステージ
FROM golang:1.21-bullseye as builder

# OpenCVの依存関係をインストール
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libopencv-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# 依存関係をコピー
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# ビルド
RUN CGO_ENABLED=1 GOOS=linux go build -o face-emotion-analyzer \
    -ldflags="-w -s" \
    cmd/server/main.go

# 実行ステージ
FROM debian:bullseye-slim

# 必要なランタイム依存関係をインストール
RUN apt-get update && apt-get install -y \
    libopencv-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# ビルドステージからバイナリとその他のファイルをコピー
COPY --from=builder /build/face-emotion-analyzer .
COPY --from=builder /build/config/config.yaml ./config/
COPY --from=builder /build/models ./models/
COPY --from=builder /build/web ./web/

# 環境変数を設定
ENV APP_ENV=production
ENV PORT=8080

# ヘルスチェックを設定
HEALTHCHECK --interval=30s --timeout=3s \
    CMD curl -f http://localhost:8080/health || exit 1

# ポートを公開
EXPOSE 8080

# 非rootユーザーを作成して切り替え
RUN useradd -r -u 1001 -g root appuser
USER appuser

# アプリケーションを実行
CMD ["./face-emotion-analyzer"]