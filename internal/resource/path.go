package resource

import (
	"os"
	"path/filepath"
	"runtime"
)

var (
	// リソースファイルの基準ディレクトリ
	BaseDir string
)

func init() {
	// 実行ファイルのディレクトリを取得
	execDir, err := os.Executable()
	if err != nil {
		execDir = "."
	}
	execDir = filepath.Dir(execDir)

	// 開発モードかどうかを判定
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		BaseDir = execDir
		return
	}

	// 開発モード時はプロジェクトルートを使用
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
		BaseDir = projectRoot
		return
	}

	// それ以外は実行ファイルのディレクトリを使用
	BaseDir = execDir
}

// 与えられたパスをベースディレクトリからの相対パスに解決します
func ResolvePath(path string) string {
	return filepath.Join(BaseDir, path)
}
