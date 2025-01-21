package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// エラーの種類を表す
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "VALIDATION_ERROR"
	ErrorTypeSecurity   ErrorType = "SECURITY_ERROR"
	ErrorTypeOpenCV     ErrorType = "OPENCV_ERROR"
	ErrorTypeAWS        ErrorType = "AWS_ERROR"
	ErrorTypeResource   ErrorType = "RESOURCE_ERROR"
	ErrorTypeUnexpected ErrorType = "UNEXPECTED_ERROR"
)

// カスタムエラー型
type Error struct {
	Type      ErrorType
	Message   string
	Code      string
	Err       error
	Stack     []Frame
	RequestID string
}

// スタックフレームを表す
type Frame struct {
	File     string
	Line     int
	Function string
}

// OpenCVのエラー定義
var (
	ErrOpenCVClosed   = errors.New("OpenCVリソースは既に解放されています")
	ErrOpenCVEmptyMat = errors.New("画像データが空です")
)

// errorインターフェースを実装
func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", e.Type, e.Message)
	if e.Code != "" {
		fmt.Fprintf(&b, " (code: %s)", e.Code)
	}
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

// errors.Unwrapのサポート
func (e *Error) Unwrap() error {
	return e.Err
}

// スタックトレースを追加
func (e *Error) WithStack() *Error {
	if len(e.Stack) > 0 {
		return e
	}

	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	stack := make([]Frame, 0, n)
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "github.com/okamyuji") {
			continue
		}
		stack = append(stack, Frame{
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
		})
		if !more {
			break
		}
	}
	e.Stack = stack
	return e
}

// 新しいエラーを作成
func NewError(errType ErrorType, message string, err error) *Error {
	e := &Error{
		Type:    errType,
		Message: message,
		Err:     err,
	}
	return e.WithStack()
}

// OpenCVのエラーを変換
func OpenCVError(operation string, err error) *Error {
	if err == nil {
		return nil
	}

	var message string
	switch err.Error() {
	case "Mat: Already closed":
		message = "OpenCVリソースは既に解放されています"
	case "Mat: Empty":
		message = "画像データが空です"
	default:
		message = fmt.Sprintf("OpenCV操作エラー: %s", operation)
	}

	return NewError(ErrorTypeOpenCV, message, err)
}

// バリデーションエラーを作成
func ValidationError(message string, err error) *Error {
	return NewError(ErrorTypeValidation, message, err)
}

// セキュリティエラーを作成
func SecurityError(message string, err error) *Error {
	return NewError(ErrorTypeSecurity, message, err)
}

// リソースエラーを作成
func ResourceError(message string, err error) *Error {
	return NewError(ErrorTypeResource, message, err)
}

// AWSエラーを作成
func AWSError(service, operation string, err error) *Error {
	message := fmt.Sprintf("AWS %s: %s failed", service, operation)
	return NewError(ErrorTypeAWS, message, err)
}

// エラーの種類を判定
func IsType(err error, errType ErrorType) bool {
	var e *Error
	if ok := As(err, &e); ok {
		return e.Type == errType
	}
	return false
}

// errors.Asのラッパー
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// errors.Isのラッパー
func Is(err, target error) bool {
	return errors.Is(err, target)
}
