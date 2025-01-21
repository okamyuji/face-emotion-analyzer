package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/okamyuji/face-emotion-analyzer/config"
	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
	"github.com/okamyuji/face-emotion-analyzer/internal/handler"
	"github.com/okamyuji/face-emotion-analyzer/internal/middleware"

	"gocv.io/x/gocv"
)

// バージョン情報
var (
	Version    = "dev"
	CommitHash = "unknown"
	BuildTime  = "unknown"
)

func main() {
	// コマンドライン引数の解析
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "バージョン情報を表示")
	flag.Parse()

	// バージョン情報の表示
	if showVersion {
		fmt.Printf("Version: %s\nCommit: %s\nBuild Time: %s\n", Version, CommitHash, BuildTime)
		return
	}

	// 環境とロギングの初期化
	logger := initEnvironment()

	// 顔認識の初期化
	cascade, err := initFaceDetection(logger)
	if err != nil {
		os.Exit(1)
	}
	defer cascade.Close()

	// ハンドラーの初期化
	if err := initHandlers(logger, &cascade); err != nil {
		os.Exit(1)
	}

	// サーバーの初期化と起動
	server := initServer(logger)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("サーバー起動失敗", "error", err)
		os.Exit(1)
	}
}

// 暗号学的に安全なランダムトークンを生成
func generateCSRFToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		slog.Error("CSRFトークンの生成に失敗", "error", err)
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// 環境変数の設定
func initEnvironment() *slog.Logger {
	// 環境変数の設定
	if os.Getenv("APP_ENV") == "" {
		os.Setenv("APP_ENV", "development")
	}

	// CSRFトークンを生成して設定
	csrfToken := generateCSRFToken()
	os.Setenv("CSRF_TOKEN", csrfToken)

	// 構造化ロギングをセットアップ
	var handler slog.Handler
	if os.Getenv("APP_ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// 顔認識の初期化
func initFaceDetection(logger *slog.Logger) (gocv.CascadeClassifier, error) {
	// 顔認識用のカスケード分類器を読み込み
	cascade := gocv.NewCascadeClassifier()
	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		logger.Error("カスケード分類器の読み込みに失敗")
		return cascade, fmt.Errorf("カスケード分類器の読み込みに失敗")
	}

	return cascade, nil
}

func initHandlers(logger *slog.Logger, cascade *gocv.CascadeClassifier) error {
	// テンプレートレンダラーの初期化
	templateRenderer, err := handler.NewTemplateRenderer("../../web/templates/*.html")
	if err != nil {
		logger.Error("テンプレートレンダラーの初期化に失敗", "error", err)
		return err
	}

	// セキュリティミドルウェアの初期化
	securityMiddleware := middleware.NewSecurityMiddleware(&config.SecurityConfig{
		AllowedOrigins:  "http://localhost:8080",
		CSRFTokenLength: 32,
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1000,
			Burst:             100,
		},
		CORS: config.CORSConfig{
			AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "X-CSRF-Token"},
			MaxAge:         86400,
		},
	})

	// 顔認識アナライザーの初期化
	cascadePtr := cascade
	analyzer := analyzer.New(cascadePtr)

	// 依存性注入を使用した顔認識ハンドラー
	faceHandler := handler.NewFaceHandler(templateRenderer, analyzer)

	// ヘルスチェックハンドラーの初期化
	healthHandler := handler.NewHealthHandler(logger)

	// ヘルスチェックエンドポイント
	http.HandleFunc("/health", healthHandler.Handle)

	// メインの顔認識ハンドラー
	http.HandleFunc("/", securityMiddleware.Middleware(faceHandler.Handle))

	// 画像アップロードエンドポイント
	http.HandleFunc("/analyze", securityMiddleware.Middleware(faceHandler.HandleAnalyze))

	// セキュリティヘッダー付きの静的ファイル配信
	fs := http.FileServer(http.Dir("../../web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", securityMiddleware.Middleware(http.HandlerFunc(fs.ServeHTTP))))

	// faviconのハンドリング
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../web/static/img/favicon.ico")
	})

	return nil
}

// サーバーの初期化
func initServer(logger *slog.Logger) *http.Server {
	logger.Info("サーバーを初期化中", "port", "8080")
	return &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
