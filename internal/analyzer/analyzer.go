package analyzer

import (
	"errors"
	"image"

	"gocv.io/x/gocv"
)

// 感情の種類を定義
type Emotion string

const (
	EmotionHappy    Emotion = "喜び"
	EmotionSad      Emotion = "悲しみ"
	EmotionAngry    Emotion = "怒り"
	EmotionNeutral  Emotion = "普通"
	EmotionSurprise Emotion = "驚き"
)

// 顔分析インターフェース
type FaceAnalyzerInterface interface {
	Analyze(imgData []byte) (*AnalysisResult, error)
}

// 顔分析エンジン
type FaceAnalyzer struct {
	cascade *gocv.CascadeClassifier
}

// 顔の領域情報
type Face struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// 分析結果
type AnalysisResult struct {
	Faces          []Face
	PrimaryEmotion Emotion
	Confidence     float64
}

// 新しい顔分析エンジンを作成
func New(cascade *gocv.CascadeClassifier) *FaceAnalyzer {
	return &FaceAnalyzer{
		cascade: cascade,
	}
}

// 画像データから顔を分析
func (fa *FaceAnalyzer) Analyze(imgData []byte) (*AnalysisResult, error) {
	img, err := gocv.IMDecode(imgData, gocv.IMReadColor)
	if err != nil {
		return nil, err
	}
	if img.Empty() {
		return nil, errors.New("画像の読み込みに失敗")
	}
	defer img.Close()

	// 画像のサイズを標準化
	resized := gocv.NewMat()
	defer resized.Close()
	gocv.Resize(img, &resized, image.Point{}, 1.0, 1.0, gocv.InterpolationLinear)

	// グレースケールに変換
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)

	// ヒストグラム平坦化で画像のコントラストを改善
	equalized := gocv.NewMat()
	defer equalized.Close()
	gocv.EqualizeHist(gray, &equalized)

	// ガウシアンブラーでノイズを除去
	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(equalized, &blurred, image.Point{X: 5, Y: 5}, 0, 0, gocv.BorderDefault)

	// コントラストを強調
	contrast := gocv.NewMat()
	defer contrast.Close()
	blurred.ConvertTo(&contrast, gocv.MatTypeCV8U)
	gocv.AddWeighted(contrast, 1.3, contrast, 0, 10, &contrast)

	// エッジを強調
	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Laplacian(contrast, &edges, gocv.MatTypeCV8U, 3, 1, 0, gocv.BorderDefault)

	// エッジを元の画像に加算
	enhanced := gocv.NewMat()
	defer enhanced.Close()
	gocv.AddWeighted(contrast, 1.2, edges, -0.3, 0, &enhanced)

	// 顔検出のパラメータを調整
	minSize := image.Point{X: gray.Cols() / 8, Y: gray.Rows() / 8}
	maxSize := image.Point{X: gray.Cols() * 7 / 8, Y: gray.Rows() * 7 / 8}

	// 複数のパラメータセットで顔検出を試みる
	var rects []image.Rectangle
	params := []struct {
		scaleFactor  float64
		minNeighbors int
	}{
		{1.05, 2}, // より細かい探索
		{1.1, 3},  // 標準的なパラメータ
		{1.15, 4}, // より粗い探索
		{1.2, 5},  // さらに粗い探索
	}

	for _, p := range params {
		detected := fa.cascade.DetectMultiScaleWithParams(
			enhanced, // 強調された画像を使用
			p.scaleFactor,
			p.minNeighbors,
			0,
			minSize,
			maxSize,
		)
		rects = append(rects, detected...)
		if len(rects) > 0 {
			break // 顔が検出されたら終了
		}
	}

	faces := make([]Face, len(rects))
	imgWidth := float64(img.Cols())
	imgHeight := float64(img.Rows())

	for i, rect := range rects {
		faces[i] = Face{
			X:      float64(rect.Min.X) / imgWidth,
			Y:      float64(rect.Min.Y) / imgHeight,
			Width:  float64(rect.Dx()) / imgWidth,
			Height: float64(rect.Dy()) / imgHeight,
		}
	}

	if len(faces) == 0 {
		return &AnalysisResult{
			Faces:          []Face{},
			PrimaryEmotion: EmotionNeutral,
			Confidence:     0,
		}, nil
	}

	// 最大の顔を分析
	var maxFace Face
	var maxArea float64
	for _, face := range faces {
		area := face.Width * face.Height
		if area > maxArea {
			maxArea = area
			maxFace = face
		}
	}

	// 感情分析
	emotion, confidence := fa.analyzeEmotion(img, maxFace)

	return &AnalysisResult{
		Faces:          faces,
		PrimaryEmotion: emotion,
		Confidence:     confidence,
	}, nil
}

// 顔画像から感情を分析
func (fa *FaceAnalyzer) analyzeEmotion(img gocv.Mat, face Face) (Emotion, float64) {
	// 画像サイズを取得
	width := img.Cols()
	height := img.Rows()

	// 顔領域を切り出し
	roi := img.Region(image.Rect(
		int(face.X*float64(width)),
		int(face.Y*float64(height)),
		int((face.X+face.Width)*float64(width)),
		int((face.Y+face.Height)*float64(height)),
	))
	defer roi.Close()

	// グレースケールに変換
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(roi, &gray, gocv.ColorBGRToGray)

	// ヒストグラム平坦化
	equalized := gocv.NewMat()
	defer equalized.Close()
	gocv.EqualizeHist(gray, &equalized)

	// 平均輝度を計算
	mean := gocv.NewMat()
	stddev := gocv.NewMat()
	defer mean.Close()
	defer stddev.Close()
	gocv.MeanStdDev(equalized, &mean, &stddev)

	brightness := mean.GetDoubleAt(0, 0)
	variation := stddev.GetDoubleAt(0, 0)

	// 輝度と変動に基づいて感情を判定
	switch {
	case variation > 80:
		return EmotionSurprise, 0.8
	case variation > 65:
		return EmotionHappy, 0.7
	case variation > 50:
		if brightness > 140 {
			return EmotionHappy, 0.6
		}
		return EmotionSad, 0.6
	case variation > 35:
		if brightness > 140 {
			return EmotionNeutral, 0.7
		}
		return EmotionAngry, 0.6
	default:
		return EmotionNeutral, 0.8
	}
}
