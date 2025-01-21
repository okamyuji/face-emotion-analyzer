package analyzer

import (
	"fmt"
	"os"
	"testing"

	"github.com/okamyuji/face-emotion-analyzer/internal/resource"
	"gocv.io/x/gocv"
)

func TestNew(t *testing.T) {
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	cascadePath := resource.ResolvePath("models/haarcascade_frontalface_default.xml")
	if !cascade.Load(cascadePath) {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade, "", "", false)
	if analyzer == nil {
		t.Error("アナライザーの作成に失敗しました")
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	// カスケード分類器の準備
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	cascadePath := resource.ResolvePath("models/haarcascade_frontalface_default.xml")
	if !cascade.Load(cascadePath) {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade, "", "", false)

	tests := []struct {
		name        string
		imgData     []byte
		wantFaces   int
		wantErr     bool
		wantEmotion Emotion
	}{
		{
			name:        "有効な画像 - 顔あり",
			imgData:     createTestImage(t),
			wantFaces:   1,
			wantErr:     false,
			wantEmotion: EmotionHappy,
		},
		{
			name:        "無効な画像データ",
			imgData:     []byte("invalid"),
			wantFaces:   0,
			wantErr:     true,
			wantEmotion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.Analyze(tt.imgData)

			if (err != nil) != tt.wantErr {
				t.Errorf("Analyze() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if len(result.Faces) != tt.wantFaces {
					t.Errorf("Analyze() got %v faces, want %v", len(result.Faces), tt.wantFaces)
				}

				if tt.wantFaces > 0 && result.PrimaryEmotion != tt.wantEmotion {
					t.Errorf("Analyze() got emotion %v, want %v", result.PrimaryEmotion, tt.wantEmotion)
				}
			}
		})
	}
}

func TestAnalyzer_AnalyzeWithDifferentSizes(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{320, 240},  // 小さいサイズ
		{640, 480},  // 中間サイズ
		{1280, 720}, // HD
	}

	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	cascadePath := resource.ResolvePath("models/haarcascade_frontalface_default.xml")
	if !cascade.Load(cascadePath) {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade, "", "", false)

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			img := createTestImage(t)
			result, err := analyzer.Analyze(img)

			if err != nil {
				t.Errorf("Analyze() error = %v", err)
				return
			}

			if result == nil {
				t.Error("Analyze() returned nil result")
				return
			}

			// 結果の検証
			if len(result.Faces) == 0 {
				t.Error("顔が検出されませんでした")
			}
		})
	}
}

func TestSaveTestImage(t *testing.T) {
	// テスト用画像を生成
	imgData := createTestImage(t)

	// ファイルに保存
	err := os.WriteFile("test_face.jpg", imgData, 0644)
	if err != nil {
		t.Fatalf("画像の保存に失敗: %v", err)
	}
}

// テストヘルパー関数

// テスト用の画像を生成
func createTestImage(tb testing.TB) []byte {
	// 実際の顔写真を読み込む
	imgPath := resource.ResolvePath("testdata/1260_1280.jpg")
	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		tb.Fatalf("テスト画像の読み込みに失敗: %v", err)
	}
	return imgData
}

// ベンチマークテスト
func BenchmarkAnalyzer_Analyze(b *testing.B) {
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	cascadePath := resource.ResolvePath("models/haarcascade_frontalface_default.xml")
	if !cascade.Load(cascadePath) {
		b.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade, "", "", false)
	img := createTestImage(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(img)
		if err != nil {
			b.Fatal(err)
		}
	}
}
