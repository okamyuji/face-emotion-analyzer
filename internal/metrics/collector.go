package metrics

import (
	"context"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// アプリケーションメトリクスを収集
type MetricsCollector struct {
	// アプリケーションメトリクス
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	errorCounter    *prometheus.CounterVec
	activeRequests  *prometheus.GaugeVec
	analysisResults *prometheus.CounterVec
	processingTime  *prometheus.HistogramVec

	// リソースメトリクス
	memoryUsage     prometheus.Gauge
	goroutineCount  prometheus.Gauge
	cpuUsage        prometheus.Gauge
	openConnections prometheus.Gauge

	// キャッシュメトリクス
	cacheHits   *prometheus.CounterVec
	cacheMisses *prometheus.CounterVec
	cacheSize   prometheus.Gauge
	cacheItems  prometheus.Gauge

	// OpenCVメトリクス
	opencvOperations *prometheus.CounterVec
	opencvErrors     *prometheus.CounterVec
	gpuUtilization   prometheus.Gauge
	gpuMemory        prometheus.Gauge
}

// 新しいメトリクスコレクターを作成
func NewMetricsCollector() *MetricsCollector {
	m := &MetricsCollector{}

	// カスタムレジストリを作成
	registry := prometheus.NewRegistry()
	factory := promauto.With(registry)

	// リクエストメトリクス
	m.requestCounter = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_requests_total",
		Help: "処理されたリクエストの総数",
	}, []string{"method", "path", "status"})

	m.requestDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "face_analyzer_request_duration_seconds",
		Help:    "リクエスト処理時間の分布",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	m.errorCounter = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_errors_total",
		Help: "エラーの総数",
	}, []string{"type", "code"})

	m.activeRequests = factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "face_analyzer_active_requests",
		Help: "現在処理中のリクエスト数",
	}, []string{"type"})

	// 分析結果メトリクス
	m.analysisResults = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_analysis_results_total",
		Help: "感情分析の結果分布",
	}, []string{"emotion", "confidence_range"})

	m.processingTime = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "face_analyzer_processing_time_seconds",
		Help:    "画像処理時間の分布",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"operation"})

	// リソースメトリクス
	m.memoryUsage = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_memory_bytes",
		Help: "使用中のメモリ量",
	})

	m.goroutineCount = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_goroutines",
		Help: "実行中のgoroutine数",
	})

	m.cpuUsage = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cpu_usage",
		Help: "CPU使用率",
	})

	m.openConnections = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_open_connections",
		Help: "オープンしているコネクション数",
	})

	// キャッシュメトリクス
	m.cacheHits = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_cache_hits_total",
		Help: "キャッシュヒット数",
	}, []string{"cache_type"})

	m.cacheMisses = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_cache_misses_total",
		Help: "キャッシュミス数",
	}, []string{"cache_type"})

	m.cacheSize = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cache_size_bytes",
		Help: "キャッシュサイズ",
	})

	m.cacheItems = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_cache_items",
		Help: "キャッシュアイテム数",
	})

	// OpenCVメトリクス
	m.opencvOperations = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_opencv_operations_total",
		Help: "OpenCV操作の総数",
	}, []string{"operation"})

	m.opencvErrors = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "face_analyzer_opencv_errors_total",
		Help: "OpenCVエラーの総数",
	}, []string{"operation", "error_type"})

	m.gpuUtilization = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_gpu_utilization",
		Help: "GPU使用率",
	})

	m.gpuMemory = factory.NewGauge(prometheus.GaugeOpts{
		Name: "face_analyzer_gpu_memory_bytes",
		Help: "GPU使用メモリ量",
	})

	// メトリクス収集を開始
	go m.collect()

	return m
}

// 定期的にメトリクスを収集
func (m *MetricsCollector) collect() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.collectResourceMetrics()
	}
}

// リソース使用状況を収集
func (m *MetricsCollector) collectResourceMetrics() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	m.memoryUsage.Set(float64(stats.Alloc))
	m.goroutineCount.Set(float64(runtime.NumGoroutine()))

	// CPU使用率の収集は別途実装が必要
	// GPUメトリクスの収集も別途実装が必要
}

// リクエストメトリクスを記録
func (m *MetricsCollector) ObserveRequest(ctx context.Context, method, path string, duration time.Duration, status int) {
	m.requestCounter.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	m.requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// エラーを記録
func (m *MetricsCollector) RecordError(errorType, code string) {
	m.errorCounter.WithLabelValues(errorType, code).Inc()
}

// 分析結果を記録
func (m *MetricsCollector) RecordAnalysis(emotion string, confidence float64) {
	confidenceRange := getConfidenceRange(confidence)
	m.analysisResults.WithLabelValues(emotion, confidenceRange).Inc()
}

// 信頼度の範囲を文字列で返す
func getConfidenceRange(confidence float64) string {
	switch {
	case confidence >= 0.9:
		return "very_high"
	case confidence >= 0.7:
		return "high"
	case confidence >= 0.5:
		return "medium"
	case confidence >= 0.3:
		return "low"
	default:
		return "very_low"
	}
}

// 処理時間を記録
func (m *MetricsCollector) RecordProcessingTime(operation string, duration time.Duration) {
	m.processingTime.WithLabelValues(operation).Observe(duration.Seconds())
}

// キャッシュ操作を記録
func (m *MetricsCollector) RecordCacheOperation(hit bool, cacheType string) {
	if hit {
		m.cacheHits.WithLabelValues(cacheType).Inc()
	} else {
		m.cacheMisses.WithLabelValues(cacheType).Inc()
	}
}

// キャッシュ統計を更新
func (m *MetricsCollector) UpdateCacheStats(size int64, items int) {
	m.cacheSize.Set(float64(size))
	m.cacheItems.Set(float64(items))
}

// OpenCV操作を記録
func (m *MetricsCollector) RecordOpenCVOperation(operation string) {
	m.opencvOperations.WithLabelValues(operation).Inc()
}

// OpenCVエラーを記録
func (m *MetricsCollector) RecordOpenCVError(operation, errorType string) {
	m.opencvErrors.WithLabelValues(operation, errorType).Inc()
}

// GPU統計を更新
func (m *MetricsCollector) UpdateGPUStats(utilization float64, memory int64) {
	m.gpuUtilization.Set(utilization)
	m.gpuMemory.Set(float64(memory))
}

// コネクション数を更新
func (m *MetricsCollector) UpdateConnectionCount(count int) {
	m.openConnections.Set(float64(count))
}
