package validator

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/okamyuji/face-emotion-analyzer/config"
)

// 画像のバリデーション
type ImageValidator struct {
	config *config.ImageConfig
}

// 新しいImageValidator
func NewImageValidator(cfg *config.ImageConfig) *ImageValidator {
	return &ImageValidator{
		config: cfg,
	}
}

// Base64エンコードされた画像を検証
func (v *ImageValidator) ValidateBase64Image(data string) error {
	// データURIスキーマの検証
	parts := strings.Split(data, ",")
	if len(parts) != 2 {
		return fmt.Errorf("不正な画像データフォーマット")
	}

	// MIMEタイプの検証
	mimeType := extractMimeType(parts[0])
	if !v.isAllowedMimeType(mimeType) {
		return fmt.Errorf("不正な画像タイプ: %s", mimeType)
	}

	// Base64デコード
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("Base64デコードエラー: %w", err)
	}

	// サイズチェック
	if int64(len(decoded)) > v.config.MaxSize {
		return fmt.Errorf("画像サイズが大きすぎます: %d bytes", len(decoded))
	}

	// 画像フォーマットと寸法の検証
	if err := v.validateImageDimensions(decoded); err != nil {
		return err
	}

	return nil
}

// 画像ファイルを検証
func (v *ImageValidator) ValidateImageFile(file *http.File) error {
	// ファイルサイズの検証
	fileInfo, err := (*file).Stat()
	if err != nil {
		return fmt.Errorf("ファイル情報の取得に失敗: %w", err)
	}

	if fileInfo.Size() > v.config.MaxSize {
		return fmt.Errorf("ファイルサイズが大きすぎます: %d bytes", fileInfo.Size())
	}

	// MIMEタイプの検証
	ext := strings.ToLower(filepath.Ext(fileInfo.Name()))
	mimeType := mime.TypeByExtension(ext)
	if !v.isAllowedMimeType(mimeType) {
		return fmt.Errorf("不正なファイル形式: %s", mimeType)
	}

	// 画像フォーマットと寸法の検証
	buf := make([]byte, 512)
	_, err = (*file).Read(buf)
	if err != nil {
		return fmt.Errorf("ファイルの読み取りに失敗: %w", err)
	}

	if err := v.validateImageDimensions(buf); err != nil {
		return err
	}

	// ファイルポインタを先頭に戻す
	_, err = (*file).Seek(0, 0)
	if err != nil {
		return fmt.Errorf("ファイルポインタのリセットに失敗: %w", err)
	}

	return nil
}

// 画像の寸法を検証
func (v *ImageValidator) validateImageDimensions(data []byte) error {
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("画像のデコードに失敗: %w", err)
	}

	if img.Width > v.config.MaxDimension || img.Height > v.config.MaxDimension {
		return fmt.Errorf("画像サイズが大きすぎます: %dx%d", img.Width, img.Height)
	}

	return nil
}

// 許可されたMIMEタイプかどうかを判定
func (v *ImageValidator) isAllowedMimeType(mimeType string) bool {
	for _, allowed := range v.config.AllowedTypes {
		if mimeType == allowed {
			return true
		}
	}
	return false
}

// データURIからMIMEタイプを抽出
func extractMimeType(dataURI string) string {
	if !strings.HasPrefix(dataURI, "data:") {
		return ""
	}

	parts := strings.Split(dataURI, ";")
	if len(parts) < 1 {
		return ""
	}

	return strings.TrimPrefix(parts[0], "data:")
}

// HTTPリクエストの検証
type RequestValidator struct {
	config *config.SecurityConfig
}

// 新しいRequestValidator
func NewRequestValidator(cfg *config.SecurityConfig) *RequestValidator {
	return &RequestValidator{
		config: cfg,
	}
}

// CSRFトークンを検証
func (v *RequestValidator) ValidateCSRFToken(token string) error {
	if token == "" {
		return fmt.Errorf("CSRFトークンが空です")
	}

	if len(token) != v.config.CSRFTokenLength {
		return fmt.Errorf("不正なCSRFトークンの長さです")
	}

	// Base64デコードを試行して有効性を確認
	_, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("不正なCSRFトークンフォーマット")
	}

	return nil
}

// Originヘッダーを検証
func (v *RequestValidator) ValidateOrigin(origin string) error {
	if origin == "" {
		return nil // 同一オリジンの場合はスキップ
	}

	allowedOrigins := strings.Split(v.config.AllowedOrigins, ",")
	for _, allowed := range allowedOrigins {
		if origin == strings.TrimSpace(allowed) {
			return nil
		}
	}

	return fmt.Errorf("許可されていないオリジン: %s", origin)
}
