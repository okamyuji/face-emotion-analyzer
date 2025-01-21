package metrics

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// CloudWatchクライアントのインターフェース
type CloudWatchClientInterface interface {
	PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}

// CloudWatchExporterの構造体
type CloudWatchExporter struct {
	client    CloudWatchClientInterface
	namespace string
	interval  time.Duration
	done      chan struct{}
}

// 新しいCloudWatchExporterを作成
func NewCloudWatchExporter(client CloudWatchClientInterface, namespace string, interval time.Duration) *CloudWatchExporter {
	return &CloudWatchExporter{
		client:    client,
		namespace: namespace,
		interval:  interval,
		done:      make(chan struct{}),
	}
}

// prometheusメトリクスをCloudWatch用の値に変換するヘルパー関数
func getMetricValue(c prometheus.Collector) float64 {
	// メトリクス値を取得するためのチャネル
	ch := make(chan prometheus.Metric, 1)
	c.Collect(ch)
	close(ch)

	// メトリクス値を取得
	var value float64
	for m := range ch {
		if metric, err := extractValue(m); err == nil {
			value = metric
			break
		}
	}
	return value
}

// メトリクスから値を抽出
func extractValue(m prometheus.Metric) (float64, error) {
	// メトリクス値を文字列として取得
	var value float64
	dto := &dto.Metric{}
	if err := m.Write(dto); err != nil {
		return 0, err
	}

	if dto.Counter != nil {
		value = *dto.Counter.Value
	} else if dto.Gauge != nil {
		value = *dto.Gauge.Value
	} else if dto.Histogram != nil {
		value = float64(dto.Histogram.GetSampleCount())
	} else if dto.Summary != nil {
		value = float64(dto.Summary.GetSampleCount())
	}

	return value, nil
}

// exportメソッド
func (e *CloudWatchExporter) export(ctx context.Context, collector *MetricsCollector) error {
	metrics := []types.MetricDatum{
		{
			MetricName: aws.String("face_analyzer_memory_bytes"),
			Value:      aws.Float64(getMetricValue(collector.memoryUsage)),
			Unit:       types.StandardUnitBytes,
		},
		{
			MetricName: aws.String("face_analyzer_goroutines"),
			Value:      aws.Float64(getMetricValue(collector.goroutineCount)),
			Unit:       types.StandardUnitCount,
		},
		{
			MetricName: aws.String("face_analyzer_cpu_usage"),
			Value:      aws.Float64(getMetricValue(collector.cpuUsage)),
			Unit:       types.StandardUnitPercent,
		},
		{
			MetricName: aws.String("face_analyzer_requests_total"),
			Value:      aws.Float64(getMetricValue(collector.requestCounter)),
			Unit:       types.StandardUnitCount,
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("Status"),
					Value: aws.String("200"),
				},
			},
		},
		{
			MetricName: aws.String("face_analyzer_errors_total"),
			Value:      aws.Float64(getMetricValue(collector.errorCounter)),
			Unit:       types.StandardUnitCount,
		},
		{
			MetricName: aws.String("face_analyzer_cache_size_bytes"),
			Value:      aws.Float64(getMetricValue(collector.cacheSize)),
			Unit:       types.StandardUnitBytes,
		},
		{
			MetricName: aws.String("face_analyzer_cache_hits_total"),
			Value:      aws.Float64(getMetricValue(collector.cacheHits.WithLabelValues("image"))),
			Unit:       types.StandardUnitCount,
		},
		{
			MetricName: aws.String("face_analyzer_cache_misses_total"),
			Value:      aws.Float64(getMetricValue(collector.cacheMisses.WithLabelValues("image"))),
			Unit:       types.StandardUnitCount,
		},
		{
			MetricName: aws.String("face_analyzer_gpu_utilization"),
			Value:      aws.Float64(getMetricValue(collector.gpuUtilization)),
			Unit:       types.StandardUnitPercent,
		},
		{
			MetricName: aws.String("face_analyzer_gpu_memory_bytes"),
			Value:      aws.Float64(getMetricValue(collector.gpuMemory)),
			Unit:       types.StandardUnitBytes,
		},
	}

	// 処理時間の統計
	processingTimeStats := e.calculateProcessingTimeStats(collector)
	metrics = append(metrics, processingTimeStats...)

	// メトリクスの送信
	input := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(e.namespace),
		MetricData: metrics,
	}

	_, err := e.client.PutMetricData(ctx, input)
	return err
}

// 処理時間の統計
func (e *CloudWatchExporter) calculateProcessingTimeStats(collector *MetricsCollector) []types.MetricDatum {
	operations := []string{"face_detection", "emotion_analysis", "image_preprocessing"}
	var stats []types.MetricDatum

	for _, op := range operations {
		// 各操作の処理時間を取得
		observer := collector.processingTime.WithLabelValues(op)
		metric := &dto.Metric{}
		if err := observer.(prometheus.Metric).Write(metric); err != nil {
			continue
		}

		if metric.Histogram != nil {
			value := float64(0)
			if metric.Histogram.SampleCount != nil && *metric.Histogram.SampleCount > 0 {
				value = *metric.Histogram.SampleSum / float64(*metric.Histogram.SampleCount)
			}

			stats = append(stats, types.MetricDatum{
				MetricName: aws.String("face_analyzer_processing_time_seconds"),
				Value:      aws.Float64(value),
				Unit:       types.StandardUnitSeconds,
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("Operation"),
						Value: aws.String(op),
					},
				},
			})
		}
	}

	return stats
}

// メトリクスの定期的なエクスポートを開始
func (e *CloudWatchExporter) Start(ctx context.Context, collector *MetricsCollector) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.export(ctx, collector); err != nil {
				// slog.Error("メトリクスのエクスポートに失敗", "error", err)
				continue
			}
		}
	}
}
