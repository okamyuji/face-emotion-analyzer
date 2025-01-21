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
	"github.com/okamyuji/face-emotion-analyzer/internal/resource"

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

	// カスケード分類器の準備
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	cascadePath := resource.ResolvePath("models/haarcascade_frontalface_default.xml")
	if !cascade.Load(cascadePath) {
		logger.Error("カスケード分類器の読み込みに失敗")
		os.Exit(1)
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

	// テンプレートレンダラーの初期化
	renderer, err := handler.NewTemplateRenderer(resource.ResolvePath("web/templates/*.html"))
	if err != nil {
		logger.Error("テンプレートレンダラーの初期化に失敗", "error", err)
		os.Exit(1)
	}

	// 顔検出器の初期化
	faceAnalyzer := analyzer.New(&cascade, "", "", false)

	// ハンドラーの初期化
	faceHandler := handler.NewFaceHandler(renderer, faceAnalyzer)
	healthHandler := handler.NewHealthHandler(logger)

	// ルーティングの設定
	mux := http.NewServeMux()
	mux.Handle("/", securityMiddleware.Middleware(faceHandler.Handle))
	mux.Handle("/analyze", securityMiddleware.Middleware(http.HandlerFunc(faceHandler.HandleAnalyze)))
	mux.HandleFunc("/health", healthHandler.Handle)

	// 静的ファイルの提供
	fs := http.FileServer(http.Dir(resource.ResolvePath("web/static")))
	mux.Handle("/static/", http.StripPrefix("/static/", securityMiddleware.Middleware(http.HandlerFunc(fs.ServeHTTP))))

	// faviconのハンドリング
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, resource.ResolvePath("web/static/img/favicon.ico"))
	})

	// サーバーの設定
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 環境変数からポートを取得
	if port := os.Getenv("PORT"); port != "" {
		server.Addr = ":" + port
	}

	// サーバーの起動
	logger.Info("サーバーを起動します", "port", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("サーバーの起動に失敗", "error", err)
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
		if err := os.Setenv("APP_ENV", "development"); err != nil {
			slog.Error("環境変数の設定に失敗", "error", err)
		}
	}

	// CSRFトークンを生成して設定
	csrfToken := generateCSRFToken()
	if err := os.Setenv("CSRF_TOKEN", csrfToken); err != nil {
		slog.Error("CSRFトークンの設定に失敗", "error", err)
	}

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
