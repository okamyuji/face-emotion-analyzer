package interfaces

import (
	"context"
	"embed"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
)

// HTTPハンドラーのインターフェース
type HTTPHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
	HandleAnalyze(w http.ResponseWriter, r *http.Request)
}

// テンプレートレンダリングのインターフェース
type TemplateRenderer interface {
	ExecuteTemplate(w http.ResponseWriter, name string, data interface{}) error
	ParseFS(fs embed.FS) error
}

// 顔分析のインターフェース
type FaceAnalyzer interface {
	Analyze(imgData []byte) (*analyzer.AnalysisResult, error)
	Close() error
}

// キャッシュのインターフェース
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(key string)
	Clear()
}

// メトリクス収集のインターフェース
type MetricsCollector interface {
	ObserveRequest(ctx context.Context, method, path string, duration time.Duration, status int)
	RecordError(errorType, code string)
	RecordAnalysis(emotion string, confidence float64)
	RecordProcessingTime(operation string, duration time.Duration)
}

// CloudWatchクライアントのインターフェース
type CloudWatchClient interface {
	PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}

// ワーカープールのインターフェース
type WorkerPool interface {
	Submit(ctx context.Context, task Task) (interface{}, error)
	Shutdown(ctx context.Context) error
	GetStats() Stats
}

// セキュリティミドルウェアのインターフェース
type SecurityMiddleware interface {
	Middleware(next http.HandlerFunc) http.HandlerFunc
	ValidateRequest(r *http.Request) error
	SetSecurityHeaders(w http.ResponseWriter)
}

// ロギングのインターフェース
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	WithContext(ctx context.Context) Logger
	WithFields(fields map[string]interface{}) Logger
}

// 設定のインターフェース
type Config interface {
	Load(path string) error
	Validate() error
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetDuration(key string) time.Duration
}

// ワーカープールで実行されるタスクを表す
type Task interface {
	Execute(ctx context.Context) (interface{}, error)
}

// ワーカープールの統計情報を表す
type Stats struct {
	ActiveWorkers  int
	PendingTasks   int
	CompletedTasks int64
	FailedTasks    int64
}
