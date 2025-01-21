package middleware

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/okamyuji/face-emotion-analyzer/config"
	"github.com/stretchr/testify/assert"
)

func TestSecurityMiddleware_Middleware(t *testing.T) {
	// テスト用の環境変数を設定
	os.Setenv("ALLOWED_ORIGINS", "http://localhost:8080")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	tests := []struct {
		name            string
		method          string
		path            string
		headers         map[string]string
		expectedStatus  int
		expectedHeaders map[string]string
		setupFunc       func(*config.SecurityConfig)
		numRequests     int // 追加：リクエスト回数
	}{
		{
			name:   "通常のGETリクエスト",
			method: http.MethodGet,
			path:   "/",
			headers: map[string]string{
				"Origin": "http://localhost:8080",
			},
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
			},
			numRequests: 1,
		},
		{
			name:           "レート制限超過",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusTooManyRequests,
			setupFunc: func(cfg *config.SecurityConfig) {
				cfg.RateLimit.RequestsPerMinute = 1
				cfg.RateLimit.Burst = 1
			},
			numRequests: 2, // バースト制限を超えるためのリクエスト回数
		},
		{
			name:   "不正なオリジン",
			method: http.MethodGet,
			path:   "/",
			headers: map[string]string{
				"Origin": "http://malicious-site.com",
			},
			expectedStatus: http.StatusForbidden,
			numRequests:    1,
		},
		{
			name:   "CORSプリフライトリクエスト",
			method: http.MethodOptions,
			path:   "/",
			headers: map[string]string{
				"Origin":                        "http://localhost:8080",
				"Access-Control-Request-Method": "POST",
			},
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "http://localhost:8080",
				"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			},
			numRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ミドルウェアの設定
			cfg := &config.SecurityConfig{
				AllowedOrigins:  "http://localhost:8080",
				CSRFTokenLength: 32,
				RateLimit: config.RateLimitConfig{
					RequestsPerMinute: 100,
					Burst:             50,
				},
			}

			if tt.setupFunc != nil {
				tt.setupFunc(cfg)
			}

			middleware := NewSecurityMiddleware(cfg)

			// テストハンドラー
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			var lastResponse *httptest.ResponseRecorder

			// 指定された回数リクエストを実行
			for i := 0; i < tt.numRequests; i++ {
				// リクエストの作成
				req := httptest.NewRequest(tt.method, tt.path, nil)
				req.RemoteAddr = "192.0.2.1:1234"
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}

				rec := httptest.NewRecorder()

				// ミドルウェアの実行
				handler := middleware.Middleware(nextHandler)
				handler.ServeHTTP(rec, req)

				lastResponse = rec
			}

			// 最後のレスポンスを検証
			assert.Equal(t, tt.expectedStatus, lastResponse.Code)

			// レスポンスヘッダーの検証
			for key, value := range tt.expectedHeaders {
				assert.Equal(t, value, lastResponse.Header().Get(key))
			}
		})
	}
}

func TestSecurityMiddleware_ValidateCSRFToken(t *testing.T) {
	cfg := &config.SecurityConfig{
		CSRFTokenLength: 32,
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1000,
			Burst:             100,
		},
	}

	middleware := NewSecurityMiddleware(cfg)

	tests := []struct {
		name           string
		method         string
		setupHeaders   func(r *http.Request)
		expectedStatus int
	}{
		{
			name:   "POSTリクエスト - 有効なトークン",
			method: http.MethodPost,
			setupHeaders: func(r *http.Request) {
				token := generateToken()
				r.Header.Set("X-CSRF-Token", token)
				r.Header.Set("X-Expected-CSRF-Token", token)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "POSTリクエスト - 無効なトークン",
			method: http.MethodPost,
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-CSRF-Token", "invalid-token")
				r.Header.Set("X-Expected-CSRF-Token", generateToken())
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "GETリクエスト - トークン不要",
			method:         http.MethodGet,
			setupHeaders:   func(r *http.Request) {},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/", nil)
			req.RemoteAddr = "192.0.2.1:1234"
			tt.setupHeaders(req)

			rec := httptest.NewRecorder()

			handler := middleware.Middleware(nextHandler)
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestSecurityMiddleware_SecurityHeaders(t *testing.T) {
	cfg := &config.SecurityConfig{
		Headers: map[string]string{
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
	}

	middleware := NewSecurityMiddleware(cfg)

	t.Run("セキュリティヘッダーの検証", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler := middleware.Middleware(nextHandler)
		handler.ServeHTTP(rec, req)

		// 基本的なセキュリティヘッダーの検証
		expectedHeaders := map[string]string{
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Referrer-Policy":           "strict-origin-when-cross-origin",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		}

		for header, expectedValue := range expectedHeaders {
			assert.Equal(t, expectedValue, rec.Header().Get(header),
				fmt.Sprintf("ヘッダー %s の値が一致しません", header))
		}

		// CSPヘッダーの検証
		cspHeader := rec.Header().Get("Content-Security-Policy")
		requiredCSPDirectives := []string{
			"default-src 'self'",
			"frame-ancestors 'none'",
			"script-src 'self'",
			"style-src 'self'",
			"img-src 'self' data: blob:",
			"media-src 'self' blob:",
		}

		for _, directive := range requiredCSPDirectives {
			assert.Contains(t, cspHeader, directive,
				fmt.Sprintf("CSPヘッダーに %s が含まれていません", directive))
		}

		// nonceの存在を確認
		assert.Contains(t, cspHeader, "'nonce-",
			"CSPヘッダーにnonceが含まれていません")
	})
}

func TestSecurityMiddleware_Logging(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	cfg := &config.SecurityConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             50,
		},
	}

	middleware := NewSecurityMiddleware(cfg)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("User-Agent", "test-agent")

	rec := httptest.NewRecorder()

	handler := middleware.Middleware(nextHandler)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "method=GET")
	assert.Contains(t, logOutput, "path=/test-path")
	assert.Contains(t, logOutput, "user_agent=test-agent")
}

func TestSecurityMiddleware_ConcurrentAccess(t *testing.T) {
	cfg := &config.SecurityConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1000,
			Burst:             100,
		},
	}

	middleware := NewSecurityMiddleware(cfg)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	totalRequests := 50

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.1:1234"
			rec := httptest.NewRecorder()

			handler := middleware.Middleware(nextHandler)
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()
	assert.Greater(t, successCount.Load(), int32(0))
}

// ベンチマークテスト
func BenchmarkSecurityMiddleware(b *testing.B) {
	cfg := &config.SecurityConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1000,
			Burst:             100,
		},
	}

	middleware := NewSecurityMiddleware(cfg)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
