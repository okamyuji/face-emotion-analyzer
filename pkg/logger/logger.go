package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/okamyuji/face-emotion-analyzer/config"
)

// Logger
type Logger struct {
	*slog.Logger
	config *config.LoggingConfig
}

// カスタムJSONハンドラ
type JSONHandler struct {
	out    io.Writer
	attrs  []slog.Attr
	fields map[string]string
}

func NewLogger(cfg *config.LoggingConfig) (*Logger, error) {
	var output io.Writer = os.Stdout
	if cfg.Output != "stdout" {
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("ログファイルのオープンに失敗: %w", err)
		}
		output = file
	}

	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = &JSONHandler{
			out:    output,
			fields: cfg.Fields,
		}
	default:
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level: parseLevel(cfg.Level),
		})
	}

	logger := &Logger{
		Logger: slog.New(handler),
		config: cfg,
	}

	return logger, nil
}

// JSONハンドラ
func (h *JSONHandler) Handle(ctx context.Context, r slog.Record) error {
	data := make(map[string]interface{})

	// 基本フィールドの設定
	data["timestamp"] = r.Time.Format(time.RFC3339)
	data["level"] = r.Level.String()
	data["message"] = r.Message

	// カスタムフィールドの追加
	for k, v := range h.fields {
		data[k] = v
	}

	// ソースコードの位置情報
	if fr := getFrame(4); fr != nil {
		data["caller"] = fmt.Sprintf("%s:%d", fr.File, fr.Line)
	}

	// 追加の属性
	r.Attrs(func(a slog.Attr) bool {
		data[a.Key] = a.Value.Any()
		return true
	})

	// エラースタックトレース
	if ctx.Value("error") != nil {
		if err, ok := ctx.Value("error").(error); ok {
			data["error"] = err.Error()
			data["stack_trace"] = getStackTrace()
		}
	}

	// JSON形式でログを出力
	encoder := json.NewEncoder(h.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (h *JSONHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *JSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &JSONHandler{
		out:    h.out,
		attrs:  append(h.attrs, attrs...),
		fields: h.fields,
	}
}

func (h *JSONHandler) WithGroup(name string) slog.Handler {
	return h
}

// ヘルパー関数
// ログレベルのパース
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ソースコードの位置情報
func getFrame(skip int) *runtime.Frame {
	pc := make([]uintptr, 1)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return nil
	}

	frame, _ := runtime.CallersFrames(pc).Next()
	return &frame
}

// エラースタックトレース
func getStackTrace() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
