package metrics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// テスト用の定数
const (
	testNamespace    = "FaceAnalyzer"
	testInterval     = 100 * time.Millisecond
	testTimeout      = 200 * time.Millisecond
	testWaitDuration = 150 * time.Millisecond
)

// モックCloudWatchクライアント
type mockCloudWatchClient struct {
	mu                 sync.RWMutex
	putMetricDataCalls int
	lastMetrics        []types.MetricDatum
	mockError          error
}

func (m *mockCloudWatchClient) PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putMetricDataCalls++
	m.lastMetrics = params.MetricData
	if m.mockError != nil {
		return nil, m.mockError
	}
	return &cloudwatch.PutMetricDataOutput{}, nil
}

func (m *mockCloudWatchClient) getCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.putMetricDataCalls
}

func (m *mockCloudWatchClient) getLastMetrics() []types.MetricDatum {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMetrics
}

// テストヘルパー関数
func setupTest(tb testing.TB) (*mockCloudWatchClient, *MetricsCollector, *CloudWatchExporter, context.CancelFunc) {
	tb.Helper()
	mockClient := &mockCloudWatchClient{}
	collector := newTestMetricsCollector()
	exporter := NewCloudWatchExporter(mockClient, testNamespace, testInterval)
	_, cancel := context.WithTimeout(context.Background(), testTimeout)
	return mockClient, collector, exporter, cancel
}

func generateTestMetrics(tb testing.TB, collector *MetricsCollector) {
	tb.Helper()
	collector.RecordAnalysis("happy", 0.95)
	collector.RecordCacheOperation(true, "image")
	collector.UpdateGPUStats(80.0, 1024*1024*1024)
	collector.RecordProcessingTime("face_detection", 100*time.Millisecond)
	collector.UpdateCacheStats(1024*1024, 100)
	collector.ObserveRequest(context.Background(), "POST", "/analyze", 100*time.Millisecond, 200)
	collector.RecordError("validation", "400")
}

func validateMetrics(t *testing.T, metrics []types.MetricDatum, expectedMetrics []string) {
	t.Helper()
	metricsFound := make(map[string]bool)
	for _, metric := range metrics {
		metricsFound[*metric.MetricName] = true
	}

	for _, expected := range expectedMetrics {
		assert.True(t, metricsFound[expected], "期待されるメトリクス %s が見つかりません", expected)
	}
}

func TestCloudWatchExporter(t *testing.T) {
	mockClient, collector, exporter, cancel := setupTest(t)
	defer cancel()

	// テストメトリクスを生成
	generateTestMetrics(t, collector)

	// エクスポーターを開始
	go exporter.Start(context.Background(), collector)

	// メトリクスが送信されるのを待機
	time.Sleep(testWaitDuration)

	// 検証
	assert.Greater(t, mockClient.getCallCount(), 0, "メトリクスが送信されていません")
	lastMetrics := mockClient.getLastMetrics()
	require.NotEmpty(t, lastMetrics, "送信されたメトリクスが空です")

	// 期待されるメトリクスを検証
	expectedMetrics := []string{
		"face_analyzer_memory_bytes",
		"face_analyzer_goroutines",
		"face_analyzer_cpu_usage",
		"face_analyzer_requests_total",
		"face_analyzer_errors_total",
		"face_analyzer_cache_size_bytes",
		"face_analyzer_cache_hits_total",
		"face_analyzer_gpu_utilization",
		"face_analyzer_gpu_memory_bytes",
		"face_analyzer_processing_time_seconds",
	}
	validateMetrics(t, lastMetrics, expectedMetrics)
}

func TestCloudWatchExporter_CacheMetrics(t *testing.T) {
	tests := []struct {
		name           string
		operations     []bool
		expectedHits   float64
		expectedMisses float64
	}{
		{
			name:           "50%ヒット率",
			operations:     []bool{true, false, true, false, true, false},
			expectedHits:   3,
			expectedMisses: 3,
		},
		{
			name:           "100%ヒット率",
			operations:     []bool{true, true, true},
			expectedHits:   3,
			expectedMisses: 0,
		},
		{
			name:           "0%ヒット率",
			operations:     []bool{false, false, false},
			expectedHits:   0,
			expectedMisses: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient, collector, exporter, cancel := setupTest(t)
			defer cancel()

			// キャッシュ操作を記録
			for _, hit := range tt.operations {
				collector.RecordCacheOperation(hit, "image")
			}

			// メトリクスをエクスポート
			err := exporter.export(context.Background(), collector)
			require.NoError(t, err)

			// メトリクスを検証
			lastMetrics := mockClient.getLastMetrics()
			require.NotEmpty(t, lastMetrics)

			// キャッシュヒット数とミス数を検証
			for _, metric := range lastMetrics {
				switch *metric.MetricName {
				case "face_analyzer_cache_hits_total":
					assert.Equal(t, tt.expectedHits, *metric.Value)
				case "face_analyzer_cache_misses_total":
					assert.Equal(t, tt.expectedMisses, *metric.Value)
				}
			}
		})
	}
}

func TestCloudWatchExporter_ProcessingTimeStats(t *testing.T) {
	mockClient, collector, exporter, cancel := setupTest(t)
	defer cancel()

	// テストデータの生成
	operations := []struct {
		name     string
		duration time.Duration
	}{
		{"face_detection", 100 * time.Millisecond},
		{"emotion_analysis", 200 * time.Millisecond},
		{"image_preprocessing", 50 * time.Millisecond},
	}

	// 処理時間の記録
	for _, op := range operations {
		collector.RecordProcessingTime(op.name, op.duration)
	}

	// メトリクスのエクスポート
	err := exporter.export(context.Background(), collector)
	assert.NoError(t, err)

	// 検証
	metrics := mockClient.getLastMetrics()
	require.NotEmpty(t, metrics)

	// 各操作のメトリクスを検証
	foundOperations := make(map[string]bool)
	for _, metric := range metrics {
		if *metric.MetricName == "face_analyzer_processing_time_seconds" {
			for _, dim := range metric.Dimensions {
				if *dim.Name == "Operation" {
					foundOperations[*dim.Value] = true
					assert.NotNil(t, metric.Value)
					assert.Greater(t, *metric.Value, float64(0))
				}
			}
		}
	}

	// すべての操作が記録されていることを確認
	for _, op := range operations {
		assert.True(t, foundOperations[op.name], "操作 %s のメトリクスが見つかりません", op.name)
	}
}

func TestCloudWatchExporter_Shutdown(t *testing.T) {
	mockClient, collector, exporter, cancel := setupTest(t)
	defer cancel()

	ctx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	// エクスポーターを開始
	go exporter.Start(ctx, collector)

	// メトリクスを記録
	generateTestMetrics(t, collector)

	// 少し待ってからシャットダウン
	time.Sleep(testInterval / 2)
	shutdownCancel()

	// シャットダウンが完了するのを待機
	time.Sleep(testInterval * 2)

	// シャットダウン後のメトリクス送信を確認
	initialCalls := mockClient.getCallCount()
	time.Sleep(testInterval)
	assert.Equal(t, initialCalls, mockClient.getCallCount(), "シャットダウン後もメトリクスが送信されています")
}

func TestCloudWatchExporter_ExportWithErrors(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
	}{
		{
			name:        "API エラー",
			mockError:   errors.New("API error"),
			expectError: true,
		},
		{
			name:        "コンテキストキャンセル",
			mockError:   context.Canceled,
			expectError: true,
		},
		{
			name:        "正常系",
			mockError:   nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockCloudWatchClient{mockError: tt.mockError}
			collector := NewMetricsCollector()
			exporter := NewCloudWatchExporter(mockClient, testNamespace, testInterval)

			generateTestMetrics(t, collector)
			err := exporter.export(context.Background(), collector)

			if tt.expectError {
				assert.Error(t, err)
				if tt.mockError != nil {
					assert.Equal(t, tt.mockError.Error(), err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCloudWatchExporter_MetricDimensions(t *testing.T) {
	mockClient, collector, exporter, cancel := setupTest(t)
	defer cancel()

	// 様々なディメンションを持つメトリクスを記録
	ctx := context.Background()
	collector.ObserveRequest(ctx, "POST", "/analyze", 100*time.Millisecond, 200)
	collector.RecordAnalysis("happy", 0.95)
	collector.RecordProcessingTime("face_detection", 100*time.Millisecond)

	err := exporter.export(ctx, collector)
	require.NoError(t, err)

	lastMetrics := mockClient.getLastMetrics()
	require.NotEmpty(t, lastMetrics)

	// ディメンションの検証
	dimensionTests := map[string][]string{
		"RequestCount":   {"Method", "Path", "Status"},
		"ProcessingTime": {"Operation"},
	}

	for _, metric := range lastMetrics {
		if expectedDims, ok := dimensionTests[*metric.MetricName]; ok {
			validateMetricDimensions(t, metric.Dimensions, expectedDims)
		}
	}
}

func validateMetricDimensions(t *testing.T, dimensions []types.Dimension, expectedNames []string) {
	t.Helper()
	dimensionsFound := make(map[string]bool)
	for _, dim := range dimensions {
		dimensionsFound[*dim.Name] = true
		assert.NotEmpty(t, *dim.Value, "ディメンション %s の値が空です", *dim.Name)
	}

	for _, expected := range expectedNames {
		assert.True(t, dimensionsFound[expected], "期待されるディメンション %s が見つかりません", expected)
	}
}

func BenchmarkCloudWatchExporter_Export(b *testing.B) {
	mockClient := &mockCloudWatchClient{}
	collector := NewMetricsCollector()
	exporter := NewCloudWatchExporter(mockClient, testNamespace, time.Second)
	ctx := context.Background()

	// テストデータを生成
	generateTestMetrics(b, collector)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := exporter.export(ctx, collector)
		if err != nil {
			b.Fatalf("エクスポートに失敗: %v", err)
		}
	}
}
