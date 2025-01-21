package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
	"github.com/okamyuji/face-emotion-analyzer/internal/middleware"
	"gocv.io/x/gocv"
)

type FaceHandler struct {
	renderer TemplateRendererInterface
	analyzer analyzer.FaceAnalyzerInterface
}

type AnalyzeRequest struct {
	Image string `json:"image"`
}

type AnalyzeResponse struct {
	Emotion        string       `json:"emotion"`
	Confidence     float64      `json:"confidence"`
	Faces          []FaceRegion `json:"faces"`
	ProcessedImage string       `json:"processedImage"` // Base64エンコードされた画像
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type FaceRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func NewFaceHandler(
	renderer TemplateRendererInterface,
	analyzer analyzer.FaceAnalyzerInterface,
) *FaceHandler {
	return &FaceHandler{
		renderer: renderer,
		analyzer: analyzer,
	}
}

// CSRFトークンを生成
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// メインページのハンドラ
func (h *FaceHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "メソッドは許可されていません", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/" {
		http.Error(w, "ページが見つかりません", http.StatusNotFound)
		return
	}

	nonce, ok := r.Context().Value(middleware.CSPNonceKey).(string)
	if !ok {
		slog.Error("CSPノンスの取得に失敗")
		http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
		return
	}

	// CSRFトークンを生成
	csrfToken := generateToken()
	w.Header().Set("X-Expected-CSRF-Token", csrfToken)

	data := TemplateData{
		CSPNonce:  nonce,
		CSRFToken: csrfToken,
	}

	if err := h.renderer.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("テンプレート実行エラー", "error", err)
		http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
	}
}

// エラーレスポンスを送信する共通関数
func sendErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		slog.Error("エラーレスポンスの送信に失敗", "error", err)
	}
}

func (h *FaceHandler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "メソッドは許可されていません")
		return
	}

	// Content-Typeの検証
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		slog.Error("不正なContent-Type", "content_type", contentType)
		sendErrorResponse(w, http.StatusBadRequest, "invalid content type")
		return
	}

	// リクエストボディの読み取り前にデバッグログ
	slog.Debug("リクエスト受信",
		"content_length", r.ContentLength,
		"content_type", contentType,
		"csrf_token", r.Header.Get("X-CSRF-Token"),
		"transfer_encoding", r.TransferEncoding)

	// リクエストボディの読み取り
	if r.Body == nil {
		slog.Error("リクエストボディが空です")
		sendErrorResponse(w, http.StatusBadRequest, "empty request body")
		return
	}

	// バッファリーダーを使用してボディを読み取り
	const maxRequestSize = 5 * 1024 * 1024 // 5MB制限
	bodyReader := http.MaxBytesReader(w, r.Body, maxRequestSize)
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		slog.Error("リクエストボディの読み取りに失敗",
			"error", err,
			"content_length", r.ContentLength,
			"transfer_encoding", r.TransferEncoding)
		sendErrorResponse(w, http.StatusBadRequest, "image size exceeds limit")
		return
	}
	defer r.Body.Close()

	// 読み取ったデータのサイズを確認
	if len(body) == 0 {
		slog.Error("リクエストボディが空です",
			"body_length", len(body),
			"content_length", r.ContentLength,
			"transfer_encoding", r.TransferEncoding)
		sendErrorResponse(w, http.StatusBadRequest, "empty request body")
		return
	}

	// 読み取ったボディの長さとプレフィックスを確認
	slog.Debug("ボディ読み取り完了",
		"body_length", len(body),
		"expected_length", r.ContentLength,
		"body_prefix", string(body[:min(100, len(body))]))

	// リクエストのデコード
	var req AnalyzeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Error("JSONのデコードに失敗",
			"error", err,
			"body_length", len(body),
			"body_start", string(body[:min(100, len(body))]))
		sendErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Base64画像データの検証と抽出
	imgData := req.Image
	if !strings.HasPrefix(imgData, "data:image/jpeg;base64,") {
		slog.Error("不正な画像データ形式")
		sendErrorResponse(w, http.StatusBadRequest, "invalid image data format")
		return
	}
	imgData = strings.TrimPrefix(imgData, "data:image/jpeg;base64,")

	// Base64デコード
	imgBytes, err := base64.StdEncoding.DecodeString(imgData)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid image data")
		return
	}

	// 画像サイズの検証
	if len(imgBytes) > maxRequestSize {
		sendErrorResponse(w, http.StatusBadRequest, "image size exceeds limit")
		return
	}

	// 画像データの検証
	if len(imgBytes) == 0 {
		sendErrorResponse(w, http.StatusBadRequest, "empty image data")
		return
	}

	// 顔分析の実行
	results, err := h.analyzer.Analyze(imgBytes)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 顔が検出されなかった場合
	if len(results.Faces) == 0 {
		response := AnalyzeResponse{
			Emotion:    "不明",
			Confidence: 0,
			Faces:      []FaceRegion{},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			sendErrorResponse(w, http.StatusInternalServerError, "response encoding failed")
		}
		return
	}

	// レスポンスの構築
	response := AnalyzeResponse{
		Emotion:    EmotionToString(results.PrimaryEmotion),
		Confidence: float64(results.Confidence),
		Faces:      make([]FaceRegion, len(results.Faces)),
	}

	// 画像の元のサイズを取得
	imgWidth := float64(0)
	imgHeight := float64(0)
	if len(results.Faces) > 0 {
		// 画像の元のサイズを取得（ProcessedImageDataから）
		img, err := gocv.IMDecode(results.ProcessedImageData, gocv.IMReadUnchanged)
		if err == nil {
			defer img.Close()
			imgWidth = float64(img.Cols())
			imgHeight = float64(img.Rows())
		}
	}

	// 座標を正規化（0-1の範囲に変換）
	for i, face := range results.Faces {
		if imgWidth > 0 && imgHeight > 0 {
			response.Faces[i] = FaceRegion{
				X:      face.X / imgWidth,
				Y:      face.Y / imgHeight,
				Width:  face.Width / imgWidth,
				Height: face.Height / imgHeight,
			}
		} else {
			// 画像サイズが取得できない場合は元の値をそのまま使用
			response.Faces[i] = FaceRegion{
				X:      face.X,
				Y:      face.Y,
				Width:  face.Width,
				Height: face.Height,
			}
		}
	}

	// 処理済み画像データをBase64エンコードしてレスポンスに追加
	if len(results.ProcessedImageData) > 0 {
		response.ProcessedImage = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(results.ProcessedImageData)
	}

	// JSONレスポンスの送信
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "response encoding failed")
		return
	}
}

// min関数の追加（ヘルパー関数）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
