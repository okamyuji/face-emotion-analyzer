package testutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/face-emotion-analyzer/config"
)

// テスト用の設定を生成
func TestConfig() *config.Config {
	return &config.Config{
		App: struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
			Env     string `yaml:"env"`
			Debug   bool   `yaml:"debug"`
		}{
			Name:    "test-app",
			Version: "1.0.0",
			Env:     "test",
			Debug:   true,
		},
		Server: config.ServerConfig{
			Port:           "8080",
			Host:           "localhost",
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		Security: config.SecurityConfig{
			AllowedOrigins:  "http://localhost:8080",
			CSRFTokenLength: 32,
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
				Burst:             50,
			},
		},
	}
}

// テスト用のHTTPサーバーを作成
func CreateTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

// テスト用の画像を生成
func CreateTestImage(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 背景を白で塗りつぶす
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.White)
		}
	}

	// 顔のような円を描画
	centerX := width / 2
	centerY := height / 2
	radius := min(width, height) / 4

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			dx := x - centerX
			dy := y - centerY
			if dx*dx+dy*dy < radius*radius {
				img.Set(x, y, color.RGBA{255, 220, 180, 255}) // 肌色
			}
		}
	}

	// JPEGにエンコード
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("画像のエンコードに失敗: %v", err)
	}

	return buf.Bytes()
}

// Base64エンコードされたテスト画像を生成
func CreateTestImageBase64(t *testing.T, width, height int) string {
	t.Helper()
	imgData := CreateTestImage(t, width, height)
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData)
}

// ファイルの読み込みを安全に行う
func ReadTestFile(filePath string, allowedDir string) ([]byte, error) {
	// パスのバリデーション
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("不正なファイルパスです: %s", filePath)
	}

	// 許可されたディレクトリ内のパスであることを確認
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("パスの解決に失敗: %w", err)
	}
	absAllowedDir, err := filepath.Abs(allowedDir)
	if err != nil {
		return nil, fmt.Errorf("許可されたディレクトリの解決に失敗: %w", err)
	}
	if !strings.HasPrefix(absPath, absAllowedDir) {
		return nil, fmt.Errorf("不正なファイルパス: 許可されていないディレクトリ")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ファイルの読み込みに失敗: %w", err)
	}
	return data, nil
}

// HTTPレスポンスを比較
func CompareResponses(t *testing.T, got, want *http.Response) {
	t.Helper()

	if got.StatusCode != want.StatusCode {
		t.Errorf("ステータスコードが異なります: got %d, want %d", got.StatusCode, want.StatusCode)
	}

	gotBody, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatalf("レスポンスボディの読み取りに失敗: %v", err)
	}
	defer got.Body.Close()

	wantBody, err := io.ReadAll(want.Body)
	if err != nil {
		t.Fatalf("期待するレスポンスボディの読み取りに失敗: %v", err)
	}
	defer want.Body.Close()

	if !bytes.Equal(gotBody, wantBody) {
		t.Errorf("レスポンスボディが異なります:\ngot  %s\nwant %s", gotBody, wantBody)
	}

	// ヘッダーの比較
	for key, wantValues := range want.Header {
		if gotValues, ok := got.Header[key]; !ok {
			t.Errorf("ヘッダー %s が存在しません", key)
		} else if !equalStringSlices(gotValues, wantValues) {
			t.Errorf("ヘッダー %s の値が異なります: got %v, want %v", key, gotValues, wantValues)
		}
	}
}

// ヘルパー関数

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
