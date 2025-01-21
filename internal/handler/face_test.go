package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
	"github.com/okamyuji/face-emotion-analyzer/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// テスト用の定数
const (
	testImageWidth  = 640
	testImageHeight = 480
	testQuality     = 90
	testToken       = "test-csrf-token"
	testNonce       = "test-nonce"
)

// テストヘルパー関数
func setupTest(tb testing.TB) (*mockTemplateRenderer, *mockFaceAnalyzer, func()) {
	tb.Helper()
	originalToken := os.Getenv("CSRF_TOKEN")
	os.Setenv("CSRF_TOKEN", testToken)

	mockRenderer := &mockTemplateRenderer{
		executeTemplateFunc: func(w http.ResponseWriter, name string, data interface{}) error {
			return nil
		},
	}
	mockAnalyzer := &mockFaceAnalyzer{
		analyzeFunc: func(imgData []byte) (*analyzer.AnalysisResult, error) {
			return &analyzer.AnalysisResult{
				Faces:          []analyzer.Face{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}},
				PrimaryEmotion: analyzer.EmotionHappy,
				Confidence:     0.9,
			}, nil
		},
	}

	cleanup := func() {
		os.Setenv("CSRF_TOKEN", originalToken)
	}

	return mockRenderer, mockAnalyzer, cleanup
}

func createTestRequest(tb testing.TB, method, path string, body interface{}) *http.Request {
	tb.Helper()
	var req *http.Request
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		require.NoError(tb, err)
		req = httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	// CSRFトークンをヘッダーに設定
	req.Header.Set("X-CSRF-Token", testToken)
	req.Header.Set("X-Expected-CSRF-Token", testToken)

	// CSPノンスをコンテキストに追加
	ctx := context.WithValue(req.Context(), middleware.CSPNonceKey, testNonce)
	return req.WithContext(ctx)
}

// モック用の構造体と関数
type mockTemplateRenderer struct {
	executeTemplateFunc func(w http.ResponseWriter, name string, data interface{}) error
	mu                  sync.RWMutex
	callCount           int
}

func (m *mockTemplateRenderer) ExecuteTemplate(w http.ResponseWriter, name string, data interface{}) error {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	return m.executeTemplateFunc(w, name, data)
}

func (m *mockTemplateRenderer) getCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

type mockFaceAnalyzer struct {
	analyzeFunc func(imgData []byte) (*analyzer.AnalysisResult, error)
	mu          sync.RWMutex
	callCount   int
}

func (m *mockFaceAnalyzer) Analyze(imgData []byte) (*analyzer.AnalysisResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	return m.analyzeFunc(imgData)
}

func (m *mockFaceAnalyzer) getCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

func TestFaceHandler_Handle(t *testing.T) {
	mockRenderer, mockAnalyzer, cleanup := setupTest(t)
	defer cleanup()

	tests := []struct {
		name            string
		method          string
		path            string
		executeTemplate func(w http.ResponseWriter, name string, data interface{}) error
		wantStatus      int
		wantTemplate    string
	}{
		{
			name:   "GETリクエスト - 成功",
			method: http.MethodGet,
			path:   "/",
			executeTemplate: func(w http.ResponseWriter, name string, data interface{}) error {
				// テンプレートデータの検証
				if td, ok := data.(TemplateData); ok {
					assert.NotEmpty(t, td.CSPNonce)
					assert.NotEmpty(t, td.CSRFToken)
				}
				return nil
			},
			wantStatus:   http.StatusOK,
			wantTemplate: "index.html",
		},
		{
			name:   "POSTリクエスト - 不許可",
			method: http.MethodPost,
			path:   "/",
			executeTemplate: func(w http.ResponseWriter, name string, data interface{}) error {
				return nil
			},
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "GETリクエスト - テンプレートエラー",
			method: http.MethodGet,
			path:   "/",
			executeTemplate: func(w http.ResponseWriter, name string, data interface{}) error {
				return errors.New("template error")
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "不正なパス",
			method: http.MethodGet,
			path:   "/invalid",
			executeTemplate: func(w http.ResponseWriter, name string, data interface{}) error {
				return nil
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRenderer.executeTemplateFunc = tt.executeTemplate
			handler := NewFaceHandler(mockRenderer, mockAnalyzer)

			req := createTestRequest(t, tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.Handle(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantTemplate != "" {
				assert.Equal(t, 1, mockRenderer.getCallCount())
			}
		})
	}
}

func TestFaceHandler_HandleAnalyze(t *testing.T) {
	mockRenderer, _, cleanup := setupTest(t)
	defer cleanup()

	// テスト用の画像データを準備
	img := createTestImage(testImageWidth, testImageHeight)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: testQuality}))
	imgData := buf.Bytes()

	tests := []struct {
		name        string
		requestBody interface{}
		analyzeFunc func(imgData []byte) (*analyzer.AnalysisResult, error)
		wantStatus  int
		wantError   string
	}{
		{
			name: "有効なリクエスト - 成功",
			requestBody: map[string]string{
				"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData),
			},
			analyzeFunc: func(imgData []byte) (*analyzer.AnalysisResult, error) {
				return &analyzer.AnalysisResult{
					Faces:          []analyzer.Face{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}},
					PrimaryEmotion: analyzer.EmotionHappy,
					Confidence:     0.9,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "無効なリクエストボディ",
			requestBody: "invalid",
			wantStatus:  http.StatusBadRequest,
			wantError:   "invalid request body",
		},
		{
			name: "無効な画像データ",
			requestBody: map[string]string{
				"image": "invalid-base64",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid image data",
		},
		{
			name: "分析エラー",
			requestBody: map[string]string{
				"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData),
			},
			analyzeFunc: func(imgData []byte) (*analyzer.AnalysisResult, error) {
				return nil, errors.New("analysis error")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "analysis error",
		},
		{
			name: "画像サイズ超過",
			requestBody: map[string]string{
				"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(make([]byte, 10*1024*1024)),
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "image size exceeds limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAnalyzer := &mockFaceAnalyzer{
				analyzeFunc: tt.analyzeFunc,
			}
			if tt.analyzeFunc == nil {
				mockAnalyzer.analyzeFunc = func(imgData []byte) (*analyzer.AnalysisResult, error) {
					return &analyzer.AnalysisResult{}, nil
				}
			}

			handler := NewFaceHandler(mockRenderer, mockAnalyzer)
			req := createTestRequest(t, http.MethodPost, "/analyze", tt.requestBody)
			rec := httptest.NewRecorder()

			handler.HandleAnalyze(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantError != "" {
				var resp ErrorResponse
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
				assert.Contains(t, resp.Error, tt.wantError)
			} else if rec.Code == http.StatusOK {
				var resp AnalyzeResponse
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
				assert.NotEmpty(t, resp.Faces)
				assert.InDelta(t, 0.9, resp.Confidence, 0.1)
			}
		})
	}
}

func TestFaceHandler_Concurrency(t *testing.T) {
	mockRenderer, mockAnalyzer, cleanup := setupTest(t)
	defer cleanup()

	handler := NewFaceHandler(mockRenderer, mockAnalyzer)
	img := createTestImage(testImageWidth, testImageHeight)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: testQuality}))
	imgData := buf.Bytes()

	requestBody := map[string]string{
		"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData),
	}

	const numGoroutines = 10
	const numRequests = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numRequests)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				req := createTestRequest(t, http.MethodPost, "/analyze", requestBody)
				rec := httptest.NewRecorder()
				handler.HandleAnalyze(rec, req)

				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("request failed with status: %d", rec.Code)
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	assert.Equal(t, numGoroutines*numRequests, mockAnalyzer.getCallCount())
}

func BenchmarkFaceHandler_HandleAnalyze(b *testing.B) {
	mockRenderer, mockAnalyzer, cleanup := setupTest(b)
	defer cleanup()

	handler := NewFaceHandler(mockRenderer, mockAnalyzer)
	img := createTestImage(testImageWidth, testImageHeight)
	var buf bytes.Buffer
	require.NoError(b, jpeg.Encode(&buf, img, &jpeg.Options{Quality: testQuality}))
	imgData := buf.Bytes()

	requestBody := map[string]string{
		"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgData),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := createTestRequest(b, http.MethodPost, "/analyze", requestBody)
			rec := httptest.NewRecorder()
			handler.HandleAnalyze(rec, req)
		}
	})
}

// テストヘルパー関数
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 背景を白で塗りつぶす
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}

	// 顔の特徴を描画
	centerX := width / 2
	centerY := height / 2
	faceRadius := min(width, height) / 4

	// 顔の輪郭
	drawEllipse(img, centerX, centerY, faceRadius, faceRadius, color.RGBA{255, 200, 200, 255})

	// 目
	eyeRadius := faceRadius / 5
	drawEllipse(img, centerX-faceRadius/2, centerY-faceRadius/4, eyeRadius, eyeRadius, color.Black)
	drawEllipse(img, centerX+faceRadius/2, centerY-faceRadius/4, eyeRadius, eyeRadius, color.Black)

	// 口
	mouthRadius := faceRadius / 3
	drawEllipse(img, centerX, centerY+faceRadius/3, mouthRadius, mouthRadius/2, color.RGBA{255, 100, 100, 255})

	return img
}

func drawEllipse(img *image.RGBA, centerX, centerY, radiusX, radiusY int, c color.Color) {
	for y := centerY - radiusY; y <= centerY+radiusY; y++ {
		for x := centerX - radiusX; x <= centerX+radiusX; x++ {
			if (float64(x-centerX)/float64(radiusX))*(float64(x-centerX)/float64(radiusX))+
				(float64(y-centerY)/float64(radiusY))*(float64(y-centerY)/float64(radiusY)) <= 1 {
				img.Set(x, y, c)
			}
		}
	}
}
