package analyzer

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

// 感情を表すための文字列型
type Emotion string

// 顔分析機能のインターフェース
type FaceAnalyzerInterface interface {
	Analyze(imgData []byte) (*AnalysisResult, error)
}

const (
	EmotionHappy    Emotion = "happy"
	EmotionNeutral  Emotion = "neutral"
	EmotionSad      Emotion = "sad"
	EmotionUnknown  Emotion = "unknown"
	EmotionSurprise Emotion = "surprise"
	EmotionAngry    Emotion = "angry"
)

// 検出された顔の領域を保持する構造体
type Face struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// 分析結果を格納する構造体
type AnalysisResult struct {
	Faces              []Face
	PrimaryEmotion     Emotion
	Confidence         float32
	ProcessedImageData []byte
}

// 顔検出・感情分析を行うための構造体
type FaceAnalyzer struct {
	cascade gocv.CascadeClassifier
	net     gocv.Net
	useDNN  bool
}

// FaceAnalyzerのインスタンスを生成するためのコンストラクタ
// protoPath, modelPathは DNN を使う場合に指定。useDNN が false の場合は無視される。
func New(cascade *gocv.CascadeClassifier, protoPath, modelPath string, useDNN bool) *FaceAnalyzer {
	net := gocv.Net{}
	if useDNN && protoPath != "" && modelPath != "" {
		net = gocv.ReadNetFromCaffe(protoPath, modelPath)
	}
	return &FaceAnalyzer{
		cascade: *cascade,
		net:     net,
		useDNN:  useDNN,
	}
}

// Analyze は画像から顔を検出し、感情を分析します
func (fa *FaceAnalyzer) Analyze(imgData []byte) (*AnalysisResult, error) {
	// 入力データのチェック
	if len(imgData) == 0 {
		return nil, fmt.Errorf("画像データが空です")
	}

	// 画像データをMatに変換
	img, err := gocv.IMDecode(imgData, gocv.IMReadColor)
	if err != nil {
		return nil, fmt.Errorf("画像のデコードに失敗: %v", err)
	}
	defer img.Close()

	// 画像が正しく読み込まれたかチェック
	if img.Empty() {
		return nil, fmt.Errorf("無効な画像データです")
	}

	// グレースケールに変換（顔検出用）
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	// 顔の検出
	minSize := image.Point{X: gray.Cols() / 8, Y: gray.Rows() / 8}
	maxSize := image.Point{X: gray.Cols() * 3 / 4, Y: gray.Rows() * 3 / 4}

	detected := fa.cascade.DetectMultiScaleWithParams(
		gray,
		1.1,
		3,
		0,
		minSize,
		maxSize,
	)

	// 結果の準備
	result := AnalysisResult{
		Faces:          make([]Face, len(detected)),
		PrimaryEmotion: EmotionUnknown,
		Confidence:     0.0,
	}

	// 処理結果を保存するための新しい画像を作成
	outputImg := img.Clone()
	defer outputImg.Close()

	// 各顔に対して処理
	for i, rect := range detected {
		// 顔領域の感情分析
		emotion := fa.analyzeEmotion(gray, Face{
			X:      float64(rect.Min.X),
			Y:      float64(rect.Min.Y),
			Width:  float64(rect.Dx()),
			Height: float64(rect.Dy()),
		})

		result.Faces[i] = Face{
			X:      float64(rect.Min.X),
			Y:      float64(rect.Min.Y),
			Width:  float64(rect.Dx()),
			Height: float64(rect.Dy()),
		}

		// 最初の顔を主要な感情として設定
		if i == 0 {
			result.PrimaryEmotion = emotion
			result.Confidence = 0.9 // TODO: 実際のスコアを計算
		}

		// 顔の周りに緑の矩形を描画
		gocv.Rectangle(&outputImg, rect, color.RGBA{0, 255, 0, 255}, 3)

		// テキストの描画位置を計算
		textPoint := image.Point{
			X: rect.Min.X,
			Y: rect.Min.Y - 10,
		}
		// 感情を画像に描画
		gocv.PutText(&outputImg, string(emotion), textPoint, gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	}

	// 処理済みの画像をエンコード
	buf, err := gocv.IMEncode(".jpg", outputImg)
	if err != nil {
		return nil, fmt.Errorf("画像のエンコードに失敗: %v", err)
	}
	result.ProcessedImageData = buf.GetBytes()

	return &result, nil
}

// 顔画像から感情を分析
func (fa *FaceAnalyzer) analyzeEmotion(img gocv.Mat, face Face) Emotion {
	// 画像サイズを取得
	width := img.Cols()
	height := img.Rows()

	// 顔領域の座標を計算（範囲チェック付き）
	x := int(face.X)
	y := int(face.Y)
	w := int(face.Width)
	h := int(face.Height)

	// 範囲が画像内に収まるように調整
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > width {
		w = width - x
	}
	if y+h > height {
		h = height - y
	}

	// 有効な領域サイズをチェック
	if w <= 0 || h <= 0 {
		return EmotionUnknown
	}

	// 顔領域を切り出し
	roi := img.Region(image.Rect(x, y, x+w, y+h))
	defer roi.Close()

	// ヒストグラム平坦化
	equalized := gocv.NewMat()
	defer equalized.Close()
	gocv.EqualizeHist(roi, &equalized)

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
		return EmotionSurprise
	case variation > 65:
		return EmotionHappy
	case variation > 50:
		if brightness > 140 {
			return EmotionHappy
		}
		return EmotionSad
	case variation > 35:
		if brightness > 140 {
			return EmotionNeutral
		}
		return EmotionAngry
	default:
		return EmotionNeutral
	}
}
