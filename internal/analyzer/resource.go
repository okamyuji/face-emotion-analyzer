package analyzer

import (
	"context"
	"fmt"
	"image"
	"log"
	"runtime"
	"sync"

	"github.com/okamyuji/face-emotion-analyzer/internal/errors"
	"gocv.io/x/gocv"
)

// OpenCVリソースを管理
type ResourceManager struct {
	mu       sync.RWMutex
	cascade  *gocv.CascadeClassifier
	gpuMat   *gocv.Mat
	useGPU   bool
	pool     *sync.Pool
	closed   bool
	maxItems int
}

// 新しいResourceManagerを作成
func NewResourceManager(cascadeFile string, useGPU bool, maxPoolSize int) (*ResourceManager, error) {
	if cascadeFile == "" {
		return nil, fmt.Errorf("カスケード分類器のファイルパスが指定されていません")
	}

	cascade := gocv.NewCascadeClassifier()
	if !cascade.Load(cascadeFile) {
		cascade.Close()
		return nil, fmt.Errorf("カスケード分類器の読み込みに失敗: %s", cascadeFile)
	}

	rm := &ResourceManager{
		cascade:  &cascade,
		useGPU:   useGPU,
		maxItems: maxPoolSize,
		pool: &sync.Pool{
			New: func() interface{} {
				return gocv.NewMat()
			},
		},
	}

	if useGPU {
		gpuMat := gocv.NewMat()
		rm.gpuMat = &gpuMat
	}

	// ファイナライザーの登録
	runtime.SetFinalizer(rm, func(rm *ResourceManager) {
		if err := rm.Close(); err != nil {
			log.Printf("リソースマネージャーのクリーンアップに失敗: %v", err)
		}
	})

	return rm, nil
}

// Matリソースを取得
func (rm *ResourceManager) AcquireMat() (*gocv.Mat, error) {
	rm.mu.RLock()
	if rm.closed {
		rm.mu.RUnlock()
		return nil, errors.ResourceError("リソースマネージャは既に終了しています", nil)
	}
	rm.mu.RUnlock()

	mat := rm.pool.Get().(*gocv.Mat)
	if mat.Empty() {
		*mat = gocv.NewMat()
	}
	return mat, nil
}

// Matリソースを解放
func (rm *ResourceManager) ReleaseMat(mat *gocv.Mat) error {
	if rm == nil {
		return fmt.Errorf("リソースマネージャーがnilです")
	}
	if mat == nil {
		return fmt.Errorf("マトリックスがnilです")
	}

	// 新しいマトリックスを作成してプールに戻す
	*mat = gocv.NewMat()
	rm.pool.Put(mat)
	return nil
}

// 画像を処理
func (rm *ResourceManager) ProcessImage(ctx context.Context, img *gocv.Mat) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.closed {
		return errors.ResourceError("リソースマネージャは既に終了しています", nil)
	}

	if rm.useGPU {
		// GPUメモリにコピー
		img.CopyTo(rm.gpuMat)
		// GPUで処理
		if err := rm.processOnGPU(ctx, rm.gpuMat); err != nil {
			return err
		}
		// 結果をCPUメモリにコピー
		rm.gpuMat.CopyTo(img)
	}

	return nil
}

// GPUで画像を処理
func (rm *ResourceManager) processOnGPU(ctx context.Context, gpuMat *gocv.Mat) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// グレースケール変換を実行
		dest := gocv.NewMat()
		defer dest.Close()
		gocv.CvtColor(*gpuMat, &dest, gocv.ColorBGRToGray)
		dest.CopyTo(gpuMat)

		// ノイズ除去
		gocv.GaussianBlur(*gpuMat, gpuMat, image.Point{X: 3, Y: 3}, 0, 0, gocv.BorderDefault)
	}
	return nil
}

// リソースを解放
func (rm *ResourceManager) Close() error {
	var errs []error

	if rm.cascade != nil {
		if err := rm.cascade.Close(); err != nil {
			errs = append(errs, fmt.Errorf("カスケード分類器のクローズに失敗: %w", err))
		}
	}

	if rm.gpuMat != nil {
		if err := rm.gpuMat.Close(); err != nil {
			errs = append(errs, fmt.Errorf("GPUマトリックスのクローズに失敗: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("リソースのクリーンアップに失敗: %v", errs)
	}
	return nil
}

// リソースが解放済みかを確認
func (rm *ResourceManager) IsClosed() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.closed
}

// リソースの状態を取得
type Status struct {
	IsGPUEnabled bool
	PoolSize     int
	IsClosed     bool
}

// リソースの状態を返す
func (rm *ResourceManager) GetStatus() Status {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return Status{
		IsGPUEnabled: rm.useGPU,
		PoolSize:     rm.maxItems,
		IsClosed:     rm.closed,
	}
}
