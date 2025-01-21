package handler

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
)

// TemplateData はテンプレートに渡すデータ構造体
type TemplateData struct {
	CSPNonce  string
	CSRFToken string
	Data      map[string]interface{}
}

// インターフェース定義
type TemplateRendererInterface interface {
	ExecuteTemplate(w http.ResponseWriter, name string, data interface{}) error
}

// 実装
type TemplateRenderer struct {
	templates *template.Template
}

func NewEmbedFSTemplateRenderer(fs embed.FS) (*TemplateRenderer, error) {
	funcMap := template.FuncMap{
		"emotionToString": EmotionToString,
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(fs, "web/templates/*.html")
	if err != nil {
		return nil, err
	}
	return &TemplateRenderer{templates: tmpl}, nil
}

func (tr *TemplateRenderer) ExecuteTemplate(w http.ResponseWriter, name string, data interface{}) error {
	var templateData TemplateData
	if d, ok := data.(TemplateData); ok {
		templateData = d
	} else if m, ok := data.(map[string]interface{}); ok {
		templateData = TemplateData{
			Data: m,
		}
	} else {
		templateData = TemplateData{
			Data: map[string]interface{}{"value": data},
		}
	}
	return tr.templates.ExecuteTemplate(w, name, templateData)
}

// FaceAnalyzer用のインターフェース
type FaceAnalyzerInterface interface {
	Analyze(data []byte) (*analyzer.AnalysisResult, error)
}

// 型変換のためのヘルパー関数を追加
func EmotionToString(emotion analyzer.Emotion) string {
	switch emotion {
	case analyzer.EmotionHappy:
		return "喜び"
	case analyzer.EmotionSad:
		return "悲しみ"
	case analyzer.EmotionAngry:
		return "怒り"
	case analyzer.EmotionNeutral:
		return "普通"
	case analyzer.EmotionSurprise:
		return "驚き"
	default:
		return "不明"
	}
}

// ファイルシステムからテンプレートを読み込む新しいレンダラーを作成
func NewTemplateRenderer(pattern string) (*TemplateRenderer, error) {
	funcMap := template.FuncMap{
		"emotionToString": EmotionToString,
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	return &TemplateRenderer{templates: tmpl}, nil
}
