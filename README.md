# Face Emotion Analyzer

顔画像から感情を分析するWebアプリケーション

## 機能

- リアルタイムの顔検出と感情分析
- 複数の顔の同時検出
- 感情分析結果の可視化
- キャッシュによるパフォーマンス最適化
- メトリクス収集とモニタリング

## 技術スタック

- Go 1.21+
- OpenCV (gocv)
- Prometheus (メトリクス)
- AWS CloudWatch (モニタリング)
- Docker & Docker Compose

## 必要条件

- Go 1.21以上
- OpenCV 4.x
- Docker & Docker Compose
- Make

## インストール

```bash
# リポジトリのクローン
git clone https://github.com/okamyuji/face-emotion-analyzer.git
cd face-emotion-analyzer

# 依存関係のインストール
make deps

# 開発用サーバーの起動
make dev
```

## 設定

`config/`ディレクトリ内の設定ファイルで以下の項目を設定できます：

- サーバー設定（ポート、タイムアウトなど）
- セキュリティ設定（CORS、レート制限など）
- 画像処理設定（最大サイズ、品質など）
- OpenCV設定（検出パラメータ）
- ロギング設定

## API エンドポイント

### メインエンドポイント

- `GET /` - メインページ（顔認識インターフェース）
- `POST /analyze` - 画像分析エンドポイント
    - リクエスト: Base64エンコードされたJPEG画像
    - レスポンス: 検出された顔の位置と感情分析結果

### システムエンドポイント

- `GET /health` - ヘルスチェックエンドポイント
    - 応答: `200 OK` - サービスが正常に動作中
- `GET /metrics` - Prometheusメトリクスエンドポイント
    - アプリケーションの各種メトリクスを提供
    - Prometheusフォーマットで出力

### 静的ファイル

- `GET /static/*` - 静的ファイル（CSS、JavaScript、画像）
    - セキュリティヘッダー付きで配信

## 開発

```bash
# テストの実行
make test

# リンター実行
make lint

# ビルド
make build

# Docker開発環境の起動
make docker-dev
```

## デプロイ

```bash
# 本番用ビルド
make build-prod

# Dockerイメージのビルド
make docker-build

# コンテナの起動
make docker-run
```

## モニタリング

- Prometheusメトリクス
    - リクエスト統計
    - 処理時間
    - エラー率
    - リソース使用状況
    - キャッシュ効率
    - GPU使用率

- CloudWatchメトリクス
    - アプリケーションメトリクス
    - インフラメトリクス
    - カスタムメトリクス

## セキュリティ

- CSRF保護
- レート制限
- セキュリティヘッダー
- CORS設定
- 入力検証

## ライセンス

MIT

## 貢献

1. Forkする
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add amazing feature'`)
4. ブランチをプッシュ (`git push origin feature/amazing-feature`)
5. Pull Requestを作成

## 作者

[okamyuji](https://github.com/okamyuji)
