package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocv.io/x/gocv"
)

func setupTestCascadeFile(t *testing.T) string {
	// テストファイルのパスを取得
	_, currentFile, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(currentFile), "../../testdata")
	cascadeFile := filepath.Join(testDataDir, "test_cascade.xml")

	// テストデータディレクトリを作成
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)

	// モックのカスケード分類器ファイルを作成
	xmlContent := `<?xml version="1.0"?>
<opencv_storage>
<cascade type_id="opencv-cascade-classifier">
  <stageType>BOOST</stageType>
  <featureType>HAAR</featureType>
  <height>24</height>
  <width>24</width>
  <stageParams>
    <maxWeakCount>1</maxWeakCount>
  </stageParams>
  <featureParams>
    <maxCatCount>2</maxCatCount>
  </featureParams>
  <stageNum>1</stageNum>
  <stages>
    <_>
      <maxWeakCount>1</maxWeakCount>
      <stageThreshold>1.</stageThreshold>
      <weakClassifiers>
        <_>
          <internalNodes>0 -1 0 1.2562000352144241e-02</internalNodes>
          <leafValues>-1. 1.</leafValues>
        </_>
      </weakClassifiers>
    </_>
  </stages>
  <features>
    <_>
      <rects>
        <_>0 0 24 24 -1.</_>
        <_>8 0 8 24 3.</_>
      </rects>
      <tilted>0</tilted>
    </_>
  </features>
</cascade>
</opencv_storage>`

	err = os.WriteFile(cascadeFile, []byte(xmlContent), 0644)
	require.NoError(t, err)

	return cascadeFile
}

func TestResourceManager_ReleaseMat(t *testing.T) {
	cascadeFile := setupTestCascadeFile(t)

	rm, err := NewResourceManager(cascadeFile, true, 1)
	require.NoError(t, err)
	defer rm.Close()

	// 正常系のテスト
	t.Run("正常なマトリックスのリリース", func(t *testing.T) {
		mat := gocv.NewMat()
		defer func() {
			if !mat.Empty() {
				mat.Close()
			}
		}()

		err := rm.ReleaseMat(&mat)
		assert.NoError(t, err)
		assert.True(t, mat.Empty())
	})

	// 異常系のテスト - nilマトリックス
	t.Run("nilマトリックスのリリース", func(t *testing.T) {
		err := rm.ReleaseMat(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "マトリックスがnilです")
	})

	// 異常系のテスト - nilリソースマネージャー
	t.Run("nilリソースマネージャーでのリリース", func(t *testing.T) {
		var nilRM *ResourceManager
		mat := gocv.NewMat()
		defer mat.Close()

		err := nilRM.ReleaseMat(&mat)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "リソースマネージャーがnilです")
	})
}

func TestResourceManager_Close(t *testing.T) {
	cascadeFile := setupTestCascadeFile(t)

	t.Run("正常なクローズ", func(t *testing.T) {
		rm, err := NewResourceManager(cascadeFile, true, 1)
		require.NoError(t, err)
		err = rm.Close()
		assert.NoError(t, err)
	})

	t.Run("2回クローズ", func(t *testing.T) {
		rm, err := NewResourceManager(cascadeFile, true, 1)
		require.NoError(t, err)
		err1 := rm.Close()
		assert.NoError(t, err1)
		err2 := rm.Close()
		assert.NoError(t, err2)
	})
}

func TestResourceManager_Finalizer(t *testing.T) {
	cascadeFile := setupTestCascadeFile(t)

	t.Run("ファイナライザーの動作確認", func(t *testing.T) {
		rm, err := NewResourceManager(cascadeFile, true, 1)
		require.NoError(t, err)

		// リソースマネージャーの状態を確認
		status := rm.GetStatus()
		assert.True(t, status.IsGPUEnabled)
		assert.Equal(t, 1, status.PoolSize)
		assert.False(t, status.IsClosed)

		// rmへの参照をなくし、GCを実行
		rm = nil
		runtime.GC()
		runtime.Gosched()
		// ファイナライザーが実行されることを期待
	})
}
