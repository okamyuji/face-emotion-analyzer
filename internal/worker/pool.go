package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ワーカーゴルーチン
type worker struct {
	pool     *Pool
	shutdown chan struct{}
}

// ワーカープールを
type Pool struct {
	tasks         chan Task
	results       chan Result
	numWorkers    int32
	maxWorkers    int32
	metrics       metrics
	isShutdown    atomic.Bool
	mu            sync.RWMutex
	wg            sync.WaitGroup
	shutdownOnce  sync.Once
	activeWorkers atomic.Int32
	minWorkers    int32
	shutdownChan  chan struct{}
}

// タスクの実行結果
type Result struct {
	Value interface{}
	Err   error
}

// ワーカーが実行するタスク
type Task struct {
	Execute  func(ctx context.Context) (interface{}, error)
	Priority int
}

// ワーカープールの統計情報
type Stats struct {
	CurrentWorkers   int32
	MaxWorkers       int32
	TasksProcessed   int64
	TasksQueued      int64
	AverageLatency   time.Duration
	ErrorRate        float64
	QueueUtilization float64
}

type metrics struct {
	tasksProcessed int64
	tasksQueued    int64
	processingTime int64 // ナノ秒単位の合計処理時間
	errors         int64
}

// ワーカーのメインループ
func (w *worker) run() {
	defer func() {
		w.pool.activeWorkers.Add(-1)
		w.pool.wg.Done()
	}()

	for {
		select {
		case <-w.pool.shutdownChan:
			return
		case task, ok := <-w.pool.tasks:
			if !ok {
				return
			}

			start := time.Now()
			result, err := task.Execute(context.Background())
			duration := time.Since(start)

			atomic.AddInt64(&w.pool.metrics.tasksProcessed, 1)
			atomic.AddInt64(&w.pool.metrics.processingTime, duration.Nanoseconds())

			if err != nil {
				atomic.AddInt64(&w.pool.metrics.errors, 1)
			}

			// シャットダウン中でないことを確認してから結果を送信
			select {
			case <-w.pool.shutdownChan:
				return
			case w.pool.results <- Result{Value: result, Err: err}:
			default:
				// 結果を送信できない場合（バッファが満杯など）はエラーをログに記録
				atomic.AddInt64(&w.pool.metrics.errors, 1)
			}
		}
	}
}

// ワーカープールの統計情報
func (p *Pool) GetStats() Stats {
	tasksProcessed := atomic.LoadInt64(&p.metrics.tasksProcessed)
	processTime := atomic.LoadInt64(&p.metrics.processingTime)
	errors := atomic.LoadInt64(&p.metrics.errors)

	var averageLatency time.Duration
	var errorRate float64

	if tasksProcessed > 0 {
		averageLatency = time.Duration(processTime / tasksProcessed)
		errorRate = float64(errors) / float64(tasksProcessed)
	}

	currentWorkers := p.activeWorkers.Load()
	maxWorkers := atomic.LoadInt32(&p.maxWorkers)
	tasksQueued := atomic.LoadInt64(&p.metrics.tasksQueued)
	queueCap := cap(p.tasks)

	var queueUtilization float64
	if queueCap > 0 {
		queueUtilization = float64(len(p.tasks)) / float64(queueCap)
	}

	return Stats{
		CurrentWorkers:   currentWorkers,
		MaxWorkers:       maxWorkers,
		TasksProcessed:   tasksProcessed,
		TasksQueued:      tasksQueued,
		AverageLatency:   averageLatency,
		ErrorRate:        errorRate,
		QueueUtilization: queueUtilization,
	}
}

// 新しいワーカープールを作成
func NewPool(minWorkers, maxWorkers int32) *Pool {
	if minWorkers <= 0 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}

	bufferSize := int(maxWorkers * 4) // バッファサイズを増やす
	p := &Pool{
		tasks:        make(chan Task, bufferSize),
		results:      make(chan Result, bufferSize),
		numWorkers:   minWorkers,
		maxWorkers:   maxWorkers,
		minWorkers:   minWorkers,
		metrics:      metrics{},
		shutdownChan: make(chan struct{}),
	}

	// 初期ワーカーの起動
	for i := int32(0); i < minWorkers; i++ {
		p.startWorker()
	}

	// ワーカー数の監視と調整を行うゴルーチンを起動
	go p.monitorWorkers()

	return p
}

// 新しいワーカーを起動
func (p *Pool) startWorker() {
	p.wg.Add(1)
	p.activeWorkers.Add(1)
	w := &worker{
		pool:     p,
		shutdown: make(chan struct{}),
	}
	go w.run()
}

// ワーカー数を監視し、必要に応じて調整
func (p *Pool) monitorWorkers() {
	ticker := time.NewTicker(50 * time.Millisecond) // より頻繁な監視
	defer ticker.Stop()

	for {
		<-ticker.C
		if p.isShutdown.Load() {
			return
		}

		queueSize := len(p.tasks)
		currentWorkers := p.activeWorkers.Load()

		// キューサイズが現在のワーカー数の75%を超えている場合、ワーカーを追加
		if float64(queueSize) > float64(currentWorkers)*0.75 && currentWorkers < p.maxWorkers {
			needed := min(p.maxWorkers-currentWorkers, int32(2))
			for i := int32(0); i < needed; i++ {
				p.startWorker()
			}
		}

		// キューが少なく、余剰ワーカーがいる場合は削減
		if queueSize < int(currentWorkers)/4 && currentWorkers > p.minWorkers {
			toRemove := min(currentWorkers-p.minWorkers, int32(1))
			p.activeWorkers.Add(-toRemove)
		}
	}
}

// 2つのint32値の小さい方
func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// ワーカープールを終了
func (p *Pool) Shutdown(ctx context.Context) error {
	var err error
	p.shutdownOnce.Do(func() {
		p.mu.Lock()
		p.isShutdown.Store(true)
		close(p.shutdownChan)
		p.mu.Unlock()

		// 残りのタスクの完了を待機
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()

		// コンテキストのタイムアウトまたはキャンセルを待機
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-done:
			p.mu.Lock()
			close(p.tasks)
			close(p.results)
			p.mu.Unlock()
		}
	})
	return err
}

// タスクを投入
func (p *Pool) Submit(ctx context.Context, task Task) (interface{}, error) {
	if task.Execute == nil {
		return nil, errors.New("task execute function is nil")
	}

	p.mu.RLock()
	if p.isShutdown.Load() {
		p.mu.RUnlock()
		return nil, errors.New("worker pool is shutdown")
	}
	p.mu.RUnlock()

	// タスクの送信
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.shutdownChan:
		return nil, errors.New("worker pool is shutting down")
	case p.tasks <- task:
		atomic.AddInt64(&p.metrics.tasksQueued, 1)
	}

	// 結果の待機
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.shutdownChan:
		return nil, errors.New("worker pool is shutting down")
	case result, ok := <-p.results:
		if !ok {
			return nil, errors.New("worker pool is shutdown")
		}
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Value, nil
	}
}
