package errors

import (
	"errors"
	"net/http"
)

// エラーコードの定義
const (
	// 入力検証エラー (4xx)
	ErrCodeInvalidInput      = "INVALID_INPUT"
	ErrCodeInvalidImage      = "INVALID_IMAGE"
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
	ErrCodeInvalidToken      = "INVALID_TOKEN"
	ErrCodeRequestTooLarge   = "REQUEST_TOO_LARGE"
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeForbidden         = "FORBIDDEN"
	ErrCodeNotFound          = "NOT_FOUND"

	// 処理エラー (5xx)
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeDatabaseError     = "DATABASE_ERROR"
	ErrCodeOpenCVError       = "OPENCV_ERROR"
	ErrCodeResourceExhausted = "RESOURCE_EXHAUSTED"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeUnavailable       = "SERVICE_UNAVAILABLE"

	// キャッシュエラー
	ErrCodeCacheMiss          = "CACHE_MISS"
	ErrCodeCacheError         = "CACHE_ERROR"
	ErrCodeCacheCapacityError = "CACHE_CAPACITY_ERROR"

	// AWS関連エラー
	ErrCodeAWSError        = "AWS_ERROR"
	ErrCodeCloudWatchError = "CLOUDWATCH_ERROR"
	ErrCodeS3Error         = "S3_ERROR"

	// セキュリティエラー
	ErrCodeCSRFError     = "CSRF_ERROR"
	ErrCodeXSSError      = "XSS_ERROR"
	ErrCodeSecurityError = "SECURITY_ERROR"
)

// エラーメッセージのテンプレート
const (
	MsgInvalidInput      = "不正な入力データです: %s"
	MsgInvalidImage      = "不正な画像フォーマットです: %s"
	MsgInvalidRequest    = "不正なリクエストです: %s"
	MsgInvalidToken      = "無効なトークンです"
	MsgRequestTooLarge   = "リクエストサイズが大きすぎます"
	MsgRateLimitExceeded = "レート制限を超過しました"
	MsgUnauthorized      = "認証が必要です"
	MsgForbidden         = "アクセスが拒否されました"
	MsgNotFound          = "リソースが見つかりません: %s"
	MsgInternalError     = "内部エラーが発生しました"
	MsgDatabaseError     = "データベースエラーが発生しました: %s"
	MsgOpenCVError       = "画像処理エラーが発生しました: %s"
	MsgResourceExhausted = "リソースが枯渇しました"
	MsgTimeout           = "処理がタイムアウトしました"
	MsgUnavailable       = "サービスが利用できません"
	MsgCacheError        = "キャッシュエラーが発生しました: %s"
	MsgAWSError          = "AWSエラーが発生しました: %s"
	MsgSecurityError     = "セキュリティエラーが発生しました: %s"
)

// HTTPステータスコードとエラーコードのマッピング
var statusCodeMap = map[string]int{
	ErrCodeInvalidInput:      http.StatusBadRequest,
	ErrCodeInvalidImage:      http.StatusBadRequest,
	ErrCodeInvalidRequest:    http.StatusBadRequest,
	ErrCodeInvalidToken:      http.StatusUnauthorized,
	ErrCodeRequestTooLarge:   http.StatusRequestEntityTooLarge,
	ErrCodeRateLimitExceeded: http.StatusTooManyRequests,
	ErrCodeUnauthorized:      http.StatusUnauthorized,
	ErrCodeForbidden:         http.StatusForbidden,
	ErrCodeNotFound:          http.StatusNotFound,
	ErrCodeInternalError:     http.StatusInternalServerError,
	ErrCodeDatabaseError:     http.StatusInternalServerError,
	ErrCodeOpenCVError:       http.StatusInternalServerError,
	ErrCodeResourceExhausted: http.StatusServiceUnavailable,
	ErrCodeTimeout:           http.StatusGatewayTimeout,
	ErrCodeUnavailable:       http.StatusServiceUnavailable,
	ErrCodeCacheError:        http.StatusInternalServerError,
	ErrCodeAWSError:          http.StatusInternalServerError,
	ErrCodeSecurityError:     http.StatusForbidden,
}

var (
	// キーが見つからない場合のエラー
	ErrKeyNotFound = errors.New("key not found")
	// 値のサイズがキャッシュの最大サイズを超えた場合のエラー
	ErrSizeExceeded = errors.New("value size exceeds cache max size")
)

// エラーコードに対応するHTTPステータスコードを返す
func GetStatusCode(code string) int {
	if status, ok := statusCodeMap[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
