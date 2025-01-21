package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_Handle(t *testing.T) {
	// テストロガーの設定
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// ハンドラーの作成
	handler := NewHealthHandler(logger)

	// テストサーバーの作成
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// ハンドラーの実行
	handler.Handle(rec, req)

	// レスポンスの検証
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}
