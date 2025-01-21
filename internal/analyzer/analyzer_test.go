package analyzer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"gocv.io/x/gocv"
)

func TestNew(t *testing.T) {
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()

	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade)
	if analyzer == nil {
		t.Error("アナライザーの作成に失敗しました")
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	// テスト用画像の生成
	img := createTestImage(t)

	// カスケード分類器の準備
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade)

	tests := []struct {
		name        string
		imgData     []byte
		wantFaces   int
		wantErr     bool
		wantEmotion Emotion
	}{
		{
			name:        "有効な画像 - 顔あり",
			imgData:     img,
			wantFaces:   1,
			wantErr:     false,
			wantEmotion: EmotionSurprise,
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
		//{1920, 1080}, // Full HD
	}

	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		t.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade)

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			img := createTestImageWithSize(t, size.width, size.height)
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
	imgData := createTestImageWithSize(t, 640, 480)

	// ファイルに保存
	err := os.WriteFile("test_face.jpg", imgData, 0644)
	if err != nil {
		t.Fatalf("画像の保存に失敗: %v", err)
	}
}

// テストヘルパー関数

// テスト用の画像を生成
func createTestImage(tb testing.TB) []byte {
	return createTestImageWithSize(tb, 640, 480)
}

// 指定したサイズのテスト用画像を生成
func createTestImageWithSize(tb testing.TB, width, height int) []byte {
	// 新しい画像を作成
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 背景をより明るいグレーで塗りつぶす
	bgColor := color.RGBA{245, 245, 245, 255}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, bgColor)
		}
	}

	// 顔の位置とサイズを計算（画像の中心に適度なサイズの顔）
	faceX := width / 2
	faceY := height / 2
	faceSize := min(width, height) * 3 / 5 // やや小さめに調整
	faceWidth := faceSize
	faceHeight := faceSize * 6 / 5 // より縦長の顔に

	// 顔の輪郭を描画
	skinColor := color.RGBA{250, 220, 190, 255}   // より明るい肌色
	shadowColor := color.RGBA{230, 200, 170, 255} // 影の色
	hairColor := color.RGBA{30, 30, 30, 255}      // より濃い黒

	// 首を描画（より細く）
	neckWidth := faceWidth / 4
	neckHeight := faceHeight / 3
	drawRect(img, faceX-neckWidth/2, faceY+faceHeight/3,
		neckWidth, neckHeight, skinColor)

	// 顔の輪郭の影を描画（より強調）
	drawEllipse(img, faceX+8, faceY+8, faceWidth/2, faceHeight/2, shadowColor)

	// メインの顔の輪郭（より明確に）
	drawEllipse(img, faceX, faceY, faceWidth/2, faceHeight/2, skinColor)

	// 顎の形状を描画（より自然に）
	drawEllipse(img, faceX, faceY+faceHeight/4, faceWidth/2-20, faceHeight/4, skinColor)

	// 頭頂部の髪（より小さく）
	drawEllipse(img, faceX, faceY-faceHeight/2+45, faceWidth/2-10, faceHeight/7, hairColor)

	// サイドの髪（より自然に）
	// 左側
	drawEllipse(img, faceX-faceWidth/2+10, faceY-faceHeight/8, faceWidth/14, faceHeight/6, hairColor)
	// 右側
	drawEllipse(img, faceX+faceWidth/2-10, faceY-faceHeight/8, faceWidth/14, faceHeight/6, hairColor)

	// 前髪（より自然な形状）
	for i := -2; i <= 2; i++ {
		offset := float64(i) * float64(faceWidth) / 14.0
		drawEllipse(img, faceX+int(offset), faceY-faceHeight/3,
			faceWidth/22, faceHeight/14, hairColor)
	}

	// 目の位置を調整（より明確に）
	eyeY := faceY - faceHeight/7
	eyeSpacing := faceWidth / 4
	eyeWidth := faceWidth / 7
	eyeHeight := faceHeight / 9

	// 左目（より大きく）
	leftEyeX := faceX - eyeSpacing
	// 白目
	drawEllipse(img, leftEyeX, eyeY, eyeWidth, eyeHeight, color.White)
	// 黒目（大きく）
	drawEllipse(img, leftEyeX, eyeY, eyeWidth*2/3, eyeHeight*2/3, color.Black)
	// ハイライト
	drawEllipse(img, leftEyeX+4, eyeY-4, 3, 3, color.White)

	// 右目（より大きく）
	rightEyeX := faceX + eyeSpacing
	// 白目
	drawEllipse(img, rightEyeX, eyeY, eyeWidth, eyeHeight, color.White)
	// 黒目（大きく）
	drawEllipse(img, rightEyeX, eyeY, eyeWidth*2/3, eyeHeight*2/3, color.Black)
	// ハイライト
	drawEllipse(img, rightEyeX+4, eyeY-4, 3, 3, color.White)

	// 眉毛（より太く、明確に）
	eyebrowY := eyeY - faceHeight/6
	eyebrowColor := color.RGBA{35, 35, 35, 255}
	// 左眉毛
	drawEllipse(img, leftEyeX, eyebrowY, int(float64(eyeWidth)*1.3), eyeHeight*2/3, eyebrowColor)
	// 右眉毛
	drawEllipse(img, rightEyeX, eyebrowY, int(float64(eyeWidth)*1.3), eyeHeight*2/3, eyebrowColor)

	// 鼻（より自然な形状）
	noseY := faceY + faceHeight/8
	noseColor := color.RGBA{240, 200, 180, 255}
	drawEllipse(img, faceX, noseY, faceWidth/10, faceHeight/10, noseColor)
	// 鼻の影（より明確に）
	shadowColor = color.RGBA{220, 180, 160, 255}
	drawEllipse(img, faceX, noseY+faceHeight/20, faceWidth/8, faceHeight/12, shadowColor)

	// 口（より自然な形状）
	mouthY := faceY + faceHeight/3
	lipColor := color.RGBA{220, 130, 130, 255}
	// 上唇
	drawEllipse(img, faceX, mouthY-3, faceWidth/4, faceHeight/14, lipColor)
	// 下唇（やや大きく）
	drawEllipse(img, faceX, mouthY+3, faceWidth/4, faceHeight/12, lipColor)

	// 画像をJPEGにエンコード（高品質）
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 100}); err != nil {
		tb.Fatalf("画像のエンコードに失敗: %v", err)
	}

	return buf.Bytes()
}

// min関数の追加
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 楕円を描画するヘルパー関数
func drawEllipse(img *image.RGBA, centerX, centerY, radiusX, radiusY int, c color.Color) {
	for x := centerX - radiusX; x <= centerX+radiusX; x++ {
		for y := centerY - radiusY; y <= centerY+radiusY; y++ {
			if x >= 0 && x < img.Bounds().Max.X && y >= 0 && y < img.Bounds().Max.Y {
				dx := float64(x-centerX) / float64(radiusX)
				dy := float64(y-centerY) / float64(radiusY)
				if (dx*dx)+(dy*dy) <= 1.0 {
					img.Set(x, y, c)
				}
			}
		}
	}
}

// 矩形を描画するヘルパー関数
func drawRect(img *image.RGBA, x, y, width, height int, c color.Color) {
	for i := x; i < x+width; i++ {
		for j := y; j < y+height; j++ {
			if i >= 0 && i < img.Bounds().Max.X && j >= 0 && j < img.Bounds().Max.Y {
				img.Set(i, j, c)
			}
		}
	}
}

// ベンチマークテスト
func BenchmarkAnalyzer_Analyze(b *testing.B) {
	cascade := gocv.NewCascadeClassifier()
	defer cascade.Close()
	if !cascade.Load("../../models/haarcascade_frontalface_default.xml") {
		b.Fatal("カスケード分類器の読み込みに失敗しました")
	}

	analyzer := New(&cascade)
	img := createTestImage(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(img)
		if err != nil {
			b.Fatal(err)
		}
	}
}
