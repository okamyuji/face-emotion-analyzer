package handler

import (
	"log/slog"
	"net/http"
)

// ヘルスチェックエンドポイントのハンドラー
type HealthHandler struct {
	logger *slog.Logger
}

// 新しいHealthHandlerを作成します
func NewHealthHandler(logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		logger: logger,
	}
}

// ヘルスチェックリクエストを処理します
func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		h.logger.Error("ヘルスチェックレスポンスの書き込みに失敗", "error", err)
	}
}
