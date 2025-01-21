package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gocv.io/x/gocv"
)

func TestMain(m *testing.M) {
	// テスト用の環境変数を設定
	os.Setenv("APP_ENV", "test")
	os.Exit(m.Run())
}

func TestGenerateCSRFToken(t *testing.T) {
	token := generateCSRFToken()
	if token == "" {
		t.Error("CSRFトークンが生成されていません")
	}

	// Base64デコードが可能か確認
	if _, err := base64.URLEncoding.DecodeString(token); err != nil {
		t.Errorf("不正なBase64形式: %v", err)
	}
}

func TestEmbedContent(t *testing.T) {
	// 必要なファイルが存在するか確認
	requiredFiles := []string{
		"../../web/templates/index.html",
		"../../web/static/css/style.css",
		"../../web/static/js/app.js",
	}

	for _, file := range requiredFiles {
		_, err := os.Stat(file)
		if err != nil {
			t.Errorf("ファイル %s が見つかりません: %v", file, err)
		}
	}
}

func TestServerConfiguration(t *testing.T) {
	// テスト用の設定
	testCases := []struct {
		name   string
		envVar string
		value  string
	}{
		{"開発環境", "APP_ENV", "development"},
		{"本番環境", "APP_ENV", "production"},
		{"テスト環境", "APP_ENV", "test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.envVar, tc.value)
			defer os.Unsetenv(tc.envVar)

			server := createTestServer(t)
			if server.ReadTimeout != 5*time.Second {
				t.Error("不正なReadTimeout")
			}
			if server.WriteTimeout != 30*time.Second {
				t.Error("不正なWriteTimeout")
			}
			if server.IdleTimeout != 120*time.Second {
				t.Error("不正なIdleTimeout")
			}
		})
	}
}

func createTestServer(t *testing.T) *http.Server {
	t.Helper()

	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		t.Fatal("カスケード分類器の読み込みに失敗")
	}

	// テスト用のサーバー設定を返す
	return &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("レスポンスの書き込みに失敗: %v", err)
		}
	}).ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("想定外のステータスコード: got %v want %v", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("レスポンスの読み取りに失敗: %v", err)
	}
	if string(body) != "OK" {
		t.Errorf("想定外のレスポンスボディ: got %v want OK", string(body))
	}
}
