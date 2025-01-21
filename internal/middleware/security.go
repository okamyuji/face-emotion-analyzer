package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/okamyuji/face-emotion-analyzer/config"
)

// セキュリティ設定
const (
	maxUploadSize     = 10 * 1024 * 1024 // 最大10MB
	maxImageDimension = 4096             // 最大画像サイズ
	nonceLength       = 32               // CSPノンスの長さ
)

// コンテキストキーのカスタム型
type contextKey string

const (
	// CSPノンスのコンテキストキー
	CSPNonceKey contextKey = "csp-nonce"
)

// セキュリティミドルウェア
type SecurityMiddleware struct {
	config  *config.SecurityConfig
	limiter *rate.Limiter
}

// CSP用のランダムなノンスを生成
func generateNonce() string {
	nonce := make([]byte, nonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(nonce)
}

// 新しいセキュリティミドルウェアを作成
func NewSecurityMiddleware(cfg *config.SecurityConfig) *SecurityMiddleware {
	if cfg == nil {
		cfg = &config.SecurityConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
				Burst:             50,
			},
		}
	}

	return &SecurityMiddleware{
		config:  cfg,
		limiter: rate.NewLimiter(rate.Limit(cfg.RateLimit.RequestsPerMinute), cfg.RateLimit.Burst),
	}
}

// ミドルウェアチェーン
func (sm *SecurityMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. セキュリティヘッダーの設定（最初に設定）
		nonce := generateNonce()
		r = r.WithContext(context.WithValue(r.Context(), CSPNonceKey, nonce))
		sm.setSecurityHeaders(w, nonce)

		// 2. レート制限
		if !sm.limiter.Allow() {
			http.Error(w, "リクエスト制限を超えました", http.StatusTooManyRequests)
			return
		}

		// 3. CORS設定
		if err := sm.handleCORS(w, r); err != nil {
			if r.Method != http.MethodOptions {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			return
		}

		// 4. CSRFトークン検証
		if err := sm.validateCSRFToken(r); err != nil {
			http.Error(w, "無効なCSRFトークン", http.StatusForbidden)
			return
		}

		// 5. アップロード制限の検証
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/analyze") {
			if err := sm.validateUpload(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// 6. リクエストログの記録
		sm.logRequest(r)

		// 7. 次のハンドラーを実行
		next.ServeHTTP(w, r)
	}
}

// セキュリティヘッダーの設定
func (sm *SecurityMiddleware) setSecurityHeaders(w http.ResponseWriter, nonce string) {
	// 基本的なセキュリティヘッダーを設定
	headers := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	// 基本ヘッダーを設定
	for header, value := range headers {
		w.Header().Set(header, value)
	}

	// CSRFトークンを生成して設定
	csrfToken := generateToken()
	w.Header().Set("X-Expected-CSRF-Token", csrfToken)

	// Content Security Policy
	csp := []string{
		"default-src 'self'",
		fmt.Sprintf("script-src 'self' 'nonce-%s' 'unsafe-inline'", nonce),
		fmt.Sprintf("style-src 'self' 'nonce-%s' 'unsafe-inline'", nonce),
		"font-src 'self' data:",
		"img-src 'self' data: blob:",
		"media-src 'self' blob:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
	}

	// CSPヘッダーを設定
	w.Header().Set("Content-Security-Policy", strings.Join(csp, "; "))

	// 設定ファイルからのカスタムヘッダーを適用（存在する場合のみ）
	if sm.config != nil && sm.config.Headers != nil {
		for header, value := range sm.config.Headers {
			w.Header().Set(header, value)
		}
	}
}

// CORS処理
func (sm *SecurityMiddleware) handleCORS(w http.ResponseWriter, r *http.Request) error {
	origin := r.Header.Get("Origin")

	// オリジンが空の場合は同一オリジンのリクエストなので許可
	if origin == "" {
		return nil
	}

	// 同一オリジンのチェック
	if origin == fmt.Sprintf("http://%s", r.Host) ||
		origin == fmt.Sprintf("https://%s", r.Host) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, X-CSRF-Token, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		return nil
	}

	// 許可されているオリジンのリスト
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "" {
		allowedOrigins = []string{} // 空の環境変数の場合は空リストとして扱う
	}

	// オリジンの検証
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			return nil
		}
	}

	return fmt.Errorf("不正なオリジン")
}

// CSRFトークンの検証
func (sm *SecurityMiddleware) validateCSRFToken(r *http.Request) error {
	// GETリクエストはスキップ
	if r.Method == http.MethodGet || r.Method == http.MethodOptions {
		return nil
	}

	// POSTリクエストの場合のみ検証
	token := r.Header.Get("X-CSRF-Token")
	expectedToken := r.Header.Get("X-Expected-CSRF-Token")

	// トークンが存在しない場合はエラー
	if token == "" || expectedToken == "" {
		return fmt.Errorf("CSRFトークンが見つかりません")
	}

	// トークンの比較（タイミング攻撃を防ぐため、一定時間の比較を使用）
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return fmt.Errorf("無効なCSRFトークン")
	}

	return nil
}

// CSRFトークンを生成
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// アップロード制限の検証
func (sm *SecurityMiddleware) validateUpload(r *http.Request) error {
	// Content-Typeの確認
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return fmt.Errorf("不正なContent-Type")
	}

	// リクエストボディのサイズ制限
	r.Body = http.MaxBytesReader(nil, r.Body, maxUploadSize)

	// リクエストボディを読み取り、保持する
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("リクエストボディの読み取りに失敗: %v", err)
	}

	// 元のボディを復元
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// JSONデコード
	var body struct {
		Image string `json:"image"`
	}

	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&body); err != nil {
		return fmt.Errorf("不正なJSONフォーマット: %v", err)
	}

	// Base64画像の検証
	if !strings.HasPrefix(body.Image, "data:image/jpeg;base64,") {
		return fmt.Errorf("不正な画像フォーマット")
	}

	return nil
}

// リクエストのログ記録
func (sm *SecurityMiddleware) logRequest(r *http.Request) {
	slog.Info("受信リクエスト",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"timestamp", time.Now().Format(time.RFC3339),
	)
}
