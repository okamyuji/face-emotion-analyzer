package metrics

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// テスト用のメトリクスコレクター作成関数
func newTestMetricsCollector() *MetricsCollector {
	m := &MetricsCollector{}

	// リクエストメトリクス
	m.requestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_requests_total",
		Help: "処理されたリクエストの総数",
	}, []string{"method", "path", "status"})

	m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "face_analyzer_request_duration_seconds",
		Help:    "リクエスト処理時間の分布",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	m.errorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_errors_total",
		Help: "エラーの総数",
	}, []string{"type", "code"})

	m.activeRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "face_analyzer_active_requests",
		Help: "現在処理中のリクエスト数",
	}, []string{"type"})

	// 分析結果メトリクス
	m.analysisResults = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_analysis_results_total",
		Help: "感情分析の結果分布",
	}, []string{"emotion", "confidence_range"})

	m.processingTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "face_analyzer_processing_time_seconds",
		Help:    "画像処理時間の分布",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"operation"})

	// リソースメトリクス
	m.memoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_memory_bytes",
		Help: "使用中のメモリ量",
	})

	m.goroutineCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_goroutines",
		Help: "実行中のgoroutine数",
	})

	m.cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cpu_usage",
		Help: "CPU使用率",
	})

	m.openConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_open_connections",
		Help: "オープンしているコネクション数",
	})

	// キャッシュメトリクス
	m.cacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_cache_hits_total",
		Help: "キャッシュヒット数",
	}, []string{"cache_type"})

	m.cacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_cache_misses_total",
		Help: "キャッシュミス数",
	}, []string{"cache_type"})

	m.cacheSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cache_size_bytes",
		Help: "キャッシュサイズ",
	})

	m.cacheItems = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cache_items",
		Help: "キャッシュアイテム数",
	})

	// OpenCVメトリクス
	m.opencvOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_opencv_operations_total",
		Help: "OpenCV操作の総数",
	}, []string{"operation"})

	m.opencvErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_opencv_errors_total",
		Help: "OpenCVエラーの総数",
	}, []string{"operation", "error_type"})

	m.gpuUtilization = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_gpu_utilization",
		Help: "GPU使用率",
	})

	m.gpuMemory = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_gpu_memory_bytes",
		Help: "GPU使用メモリ量",
	})

	return m
}

func TestMetricsCollector_ObserveRequest(t *testing.T) {
	collector := newTestMetricsCollector()

	tests := []struct {
		name     string
		method   string
		path     string
		duration time.Duration
		status   int
	}{
		{
			name:     "正常なリクエスト",
			method:   "POST",
			path:     "/analyze",
			duration: 100 * time.Millisecond,
			status:   200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// リクエストを記録
			collector.ObserveRequest(context.Background(), tt.method, tt.path, tt.duration, tt.status)

			// リクエストカウンターの検証
			counter := collector.requestCounter.WithLabelValues(tt.method, tt.path, strconv.Itoa(tt.status))
			value := testutil.ToFloat64(counter)
			if value != 1 {
				t.Errorf("リクエスト数が不正: got %v, want 1", value)
			}

			// リクエスト時間の検証
			histogram := collector.requestDuration.WithLabelValues(tt.method, tt.path)
			if histogram == nil {
				t.Error("リクエスト時間のヒストグラムが取得できません")
			}
		})
	}
}

func TestMetricsCollector_RecordError(t *testing.T) {
	collector := newTestMetricsCollector()

	// エラーを記録
	collector.RecordError("validation", "invalid_input")

	// エラーカウンターのチェック
	count := testutil.ToFloat64(collector.errorCounter.WithLabelValues("validation", "invalid_input"))
	if count != 1 {
		t.Errorf("エラーカウントが不正: got %v, want 1", count)
	}
}

func TestMetricsCollector_RecordAnalysis(t *testing.T) {
	collector := newTestMetricsCollector()

	tests := []struct {
		name       string
		emotion    string
		confidence float64
		expected   string
	}{
		{
			name:       "高信頼度",
			emotion:    "happy",
			confidence: 0.95,
			expected:   "very_high",
		},
		{
			name:       "中程度信頼度",
			emotion:    "neutral",
			confidence: 0.65,
			expected:   "medium",
		},
		{
			name:       "低信頼度",
			emotion:    "sad",
			confidence: 0.25,
			expected:   "very_low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.RecordAnalysis(tt.emotion, tt.confidence)

			count := testutil.ToFloat64(collector.analysisResults.WithLabelValues(tt.emotion, tt.expected))
			if count != 1 {
				t.Errorf("分析結果カウントが不正: got %v, want 1", count)
			}
		})
	}
}

func TestMetricsCollector_ResourceMetrics(t *testing.T) {
	collector := newTestMetricsCollector()

	// リソースメトリクスの収集をトリガー
	collector.collectResourceMetrics()

	// メモリ使用量のチェック
	memoryUsage := testutil.ToFloat64(collector.memoryUsage)
	if memoryUsage == 0 {
		t.Error("メモリ使用量が記録されていません")
	}

	// Goroutine数のチェック
	goroutineCount := testutil.ToFloat64(collector.goroutineCount)
	if goroutineCount == 0 {
		t.Error("Goroutine数が記録されていません")
	}
}

func TestMetricsCollector_CacheOperations(t *testing.T) {
	collector := newTestMetricsCollector()

	tests := []struct {
		name      string
		hit       bool
		cacheType string
	}{
		{
			name:      "キャッシュヒット",
			hit:       true,
			cacheType: "image",
		},
		{
			name:      "キャッシュミス",
			hit:       false,
			cacheType: "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.RecordCacheOperation(tt.hit, tt.cacheType)

			var counter prometheus.Counter
			if tt.hit {
				counter = collector.cacheHits.WithLabelValues(tt.cacheType)
			} else {
				counter = collector.cacheMisses.WithLabelValues(tt.cacheType)
			}

			count := testutil.ToFloat64(counter)
			if count != 1 {
				t.Errorf("キャッシュ操作カウントが不正: got %v, want 1", count)
			}
		})
	}

	// キャッシュ統計の更新テスト
	collector.UpdateCacheStats(1024, 10)

	size := testutil.ToFloat64(collector.cacheSize)
	if size != 1024 {
		t.Errorf("キャッシュサイズが不正: got %v, want 1024", size)
	}

	items := testutil.ToFloat64(collector.cacheItems)
	if items != 10 {
		t.Errorf("キャッシュアイテム数が不正: got %v, want 10", items)
	}
}

func TestMetricsCollector_OpenCVOperations(t *testing.T) {
	collector := newTestMetricsCollector()

	// OpenCV操作の記録
	operation := "face_detection"
	collector.RecordOpenCVOperation(operation)

	count := testutil.ToFloat64(collector.opencvOperations.WithLabelValues(operation))
	if count != 1 {
		t.Errorf("OpenCV操作カウントが不正: got %v, want 1", count)
	}

	// OpenCVエラーの記録
	errorType := "initialization_error"
	collector.RecordOpenCVError(operation, errorType)

	errorCount := testutil.ToFloat64(collector.opencvErrors.WithLabelValues(operation, errorType))
	if errorCount != 1 {
		t.Errorf("OpenCVエラーカウントが不正: got %v, want 1", errorCount)
	}
}

func TestMetricsCollector_GPUStats(t *testing.T) {
	collector := newTestMetricsCollector()

	// GPU統計の更新
	utilization := 75.5
	memory := int64(1024 * 1024 * 1024) // 1GB
	collector.UpdateGPUStats(utilization, memory)

	util := testutil.ToFloat64(collector.gpuUtilization)
	if util != utilization {
		t.Errorf("GPU使用率が不正: got %v, want %v", util, utilization)
	}

	mem := testutil.ToFloat64(collector.gpuMemory)
	if mem != float64(memory) {
		t.Errorf("GPUメモリが不正: got %v, want %v", mem, memory)
	}
}

func TestMetricsCollector_ProcessingTime(t *testing.T) {
	collector := newTestMetricsCollector()

	operation := "image_preprocessing"
	duration := 100 * time.Millisecond

	collector.RecordProcessingTime(operation, duration)
}

func TestMetricsCollector_ConnectionTracking(t *testing.T) {
	collector := newTestMetricsCollector()

	// コネクション数の更新
	connections := 42
	collector.UpdateConnectionCount(connections)

	count := testutil.ToFloat64(collector.openConnections)
	if count != float64(connections) {
		t.Errorf("コネクション数が不正: got %v, want %v", count, connections)
	}
}

// ベンチマーク
func BenchmarkMetricsCollector(b *testing.B) {
	collector := newTestMetricsCollector()
	ctx := context.Background()

	b.Run("ObserveRequest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.ObserveRequest(ctx, "POST", "/analyze", 100*time.Millisecond, 200)
		}
	})

	b.Run("RecordAnalysis", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.RecordAnalysis("happy", 0.95)
		}
	})

	b.Run("CacheOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.RecordCacheOperation(i%2 == 0, "image")
		}
	})
}
