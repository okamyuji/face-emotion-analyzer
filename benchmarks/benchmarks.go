package benchmarks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/okamyuji/face-emotion-analyzer/internal/analyzer"
	"github.com/okamyuji/face-emotion-analyzer/internal/handler"
	"gocv.io/x/gocv"
)

// ベンチマーク用のヘルパー関数
func generateTestImage(b *testing.B, width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// テスト用の顔のような円を描画
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			// 中心からの距離を計算
			dx := float64(x - width/2)
			dy := float64(y - height/2)
			distance := dx*dx + dy*dy
			radius := float64((width + height) / 8)

			if distance < radius*radius {
				img.Set(x, y, color.RGBA{255, 220, 180, 255}) // 肌色
			} else {
				img.Set(x, y, color.RGBA{200, 200, 200, 255}) // 背景色
			}
		}
	}

	// 画像をJPEGにエンコード
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		b.Fatal(err)
	}

	return buf.Bytes()
}

// シングルスレッドでのベンチマーク
func BenchmarkFaceAnalysis(b *testing.B) {
	// OpenCVの初期化
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗")
	}

	// アナライザーの初期化
	analyzer := analyzer.New(&cascade, "", "", false)

	// テスト画像の生成
	imgData := generateTestImage(b, 640, 480)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(imgData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 並行処理のベンチマーク
func BenchmarkConcurrentFaceAnalysis(b *testing.B) {
	// OpenCVの初期化
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗")
	}

	// アナライザーの初期化
	analyzer := analyzer.New(&cascade, "", "", false)

	// テスト画像の生成
	imgData := generateTestImage(b, 640, 480)

	// 並行処理用のチャネル
	type result struct {
		err error
	}
	results := make(chan result)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func() {
			_, err := analyzer.Analyze(imgData)
			results <- result{err: err}
		}()
	}

	// 結果の収集
	for i := 0; i < b.N; i++ {
		r := <-results
		if r.err != nil {
			b.Fatal(r.err)
		}
	}
}

// HTTPエンドポイントのベンチマーク
func BenchmarkHTTPEndpoint(b *testing.B) {
	// サーバーの設定
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗")
	}

	analyzer := analyzer.New(&cascade, "", "", false)
	handler := handler.NewFaceHandler(nil, analyzer)

	// テストサーバー
	ts := httptest.NewServer(http.HandlerFunc(handler.HandleAnalyze))
	defer ts.Close()

	// テスト画像の準備
	imgData := generateTestImage(b, 640, 480)
	base64Img := base64.StdEncoding.EncodeToString(imgData)
	requestBody, err := json.Marshal(map[string]string{
		"image": "data:image/jpeg;base64," + base64Img,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(requestBody))
		if err != nil {
			b.Fatal(err)
		}
		if err := resp.Body.Close(); err != nil {
			b.Errorf("レスポンスボディのクローズに失敗: %v", err)
		}
	}
}

// メモリ使用量のベンチマーク
func BenchmarkMemoryUsage(b *testing.B) {
	b.ReportAllocs()

	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗")
	}

	analyzer := analyzer.New(&cascade, "", "", false)

	// 異なるサイズの画像でテスト
	sizes := []struct {
		width  int
		height int
	}{
		{320, 240},   // 小さいサイズ
		{640, 480},   // 中間サイズ
		{1280, 720},  // HD
		{1920, 1080}, // Full HD
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(b *testing.B) {
			imgData := generateTestImage(b, size.width, size.height)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.Analyze(imgData)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// タイムアウト処理のベンチマーク
func BenchmarkTimeoutHandling(b *testing.B) {
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗")
	}

	analyzer := analyzer.New(&cascade, "", "", false)
	imgData := generateTestImage(b, 1920, 1080) // 大きなサイズの画像を使用

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_, err := analyzer.Analyze(imgData)
			if err != nil {
				b.Error(err)
			}
			close(done)
		}()

		select {
		case <-ctx.Done():
			b.Fatal("タイムアウト")
		case <-done:
			// 正常完了
		}
	}
}
