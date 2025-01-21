package worker

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	t.Run("基本的なタスク実行", func(t *testing.T) {
		pool := NewPool(2, 4)
		defer func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("pool.Shutdown() error = %v", err)
			}
		}()

		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				return "success", nil
			},
			Priority: 1,
		}

		result, err := pool.Submit(context.Background(), task)
		if err != nil {
			t.Errorf("タスク実行エラー: %v", err)
		}
		if result != "success" {
			t.Errorf("予期しない結果: got %v, want success", result)
		}
	})

	t.Run("コンテキストキャンセル", func(t *testing.T) {
		pool := NewPool(2, 4)
		defer func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("pool.Shutdown() error = %v", err)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 即座にキャンセル

		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				time.Sleep(time.Second)
				return nil, nil
			},
		}

		_, err := pool.Submit(ctx, task)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("コンテキストキャンセルエラーを期待: got %v", err)
		}
	})

	t.Run("並行タスク実行", func(t *testing.T) {
		pool := NewPool(4, 8)
		defer func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("pool.Shutdown() error = %v", err)
			}
		}()

		var wg sync.WaitGroup
		numTasks := 100
		results := make([]string, numTasks)
		wg.Add(numTasks)

		for i := 0; i < numTasks; i++ {
			i := i
			go func() {
				defer wg.Done()
				task := Task{
					Execute: func(ctx context.Context) (interface{}, error) {
						return "task-" + string(rune(i)), nil
					},
				}
				result, err := pool.Submit(context.Background(), task)
				if err != nil {
					t.Errorf("タスク%d実行エラー: %v", i, err)
				}
				results[i] = result.(string)
			}()
		}

		wg.Wait()
		stats := pool.GetStats()
		if stats.TasksProcessed != int64(numTasks) {
			t.Errorf("処理タスク数不一致: got %d, want %d", stats.TasksProcessed, numTasks)
		}
	})

	t.Run("エラー処理", func(t *testing.T) {
		pool := NewPool(2, 4)
		defer func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("pool.Shutdown() error = %v", err)
			}
		}()

		expectedErr := errors.New("test error")
		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				return nil, expectedErr
			},
		}

		_, err := pool.Submit(context.Background(), task)
		if !errors.Is(err, expectedErr) {
			t.Errorf("期待するエラーではありません: got %v, want %v", err, expectedErr)
		}

		stats := pool.GetStats()
		if stats.ErrorRate == 0 {
			t.Error("エラー率が0になっています")
		}
	})

	t.Run("ワーカー数の動的調整", func(t *testing.T) {
		pool := NewPool(2, 8)
		defer func() {
			if err := pool.Shutdown(context.Background()); err != nil {
				t.Errorf("pool.Shutdown() error = %v", err)
			}
		}()

		// 大量のタスクを投入して負荷をかける
		var wg sync.WaitGroup
		numTasks := 100
		wg.Add(numTasks)

		for i := 0; i < numTasks; i++ {
			go func() {
				defer wg.Done()
				task := Task{
					Execute: func(ctx context.Context) (interface{}, error) {
						time.Sleep(10 * time.Millisecond)
						return nil, nil
					},
				}
				_, _ = pool.Submit(context.Background(), task)
			}()
		}

		// ワーカー数が増加することを確認
		time.Sleep(100 * time.Millisecond)
		stats := pool.GetStats()
		if stats.CurrentWorkers <= 2 {
			t.Error("ワーカー数が増加していません")
		}

		wg.Wait()

		// 負荷が下がったらワーカー数が減少することを確認
		time.Sleep(200 * time.Millisecond)
		stats = pool.GetStats()
		if stats.CurrentWorkers > 4 {
			t.Error("ワーカー数が減少していません")
		}
	})

	t.Run("シャットダウン", func(t *testing.T) {
		pool := NewPool(2, 4)
		var wg sync.WaitGroup
		numTasks := 10
		wg.Add(numTasks)

		// タスクの完了を追跡
		taskResults := make(chan struct{}, numTasks)

		// すべてのタスクを一度に投入
		for i := 0; i < numTasks; i++ {
			task := Task{
				Execute: func(ctx context.Context) (interface{}, error) {
					time.Sleep(50 * time.Millisecond)
					return "completed", nil
				},
			}

			// タスクを同期的に投入
			result, err := pool.Submit(context.Background(), task)
			if err != nil {
				t.Errorf("タスク投入エラー: %v", err)
			}
			if result != "completed" {
				t.Errorf("予期しない結果: got %v, want completed", result)
			}
			taskResults <- struct{}{}
			wg.Done()
		}

		// タスクの完了を待機
		go func() {
			wg.Wait()
			close(taskResults)
		}()

		// すべてのタスクが完了するまで待機
		for range taskResults {
			// タスクの完了を確認
		}

		// シャットダウンを開始
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := pool.Shutdown(ctx); err != nil {
			t.Errorf("シャットダウンエラー: %v", err)
		}

		// シャットダウン後のタスク投入を確認
		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				return nil, nil
			},
		}
		_, err := pool.Submit(context.Background(), task)
		if err == nil {
			t.Error("シャットダウン後にタスクが受け付けられました")
		} else if !strings.Contains(err.Error(), "shutdown") {
			t.Errorf("予期しないエラー: %v", err)
		}
	})
}

func BenchmarkWorkerPool(b *testing.B) {
	pool := NewPool(int32(runtime.NumCPU()), int32(runtime.NumCPU()*2))
	defer func() {
		if err := pool.Shutdown(context.Background()); err != nil {
			b.Errorf("pool.Shutdown() error = %v", err)
		}
	}()

	b.Run("単純なタスク", func(b *testing.B) {
		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				return nil, nil
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Submit(context.Background(), task)
		}
	})

	b.Run("計算タスク", func(b *testing.B) {
		task := Task{
			Execute: func(ctx context.Context) (interface{}, error) {
				result := 0
				for i := 0; i < 1000; i++ {
					result += i
				}
				return result, nil
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Submit(context.Background(), task)
		}
	})

	b.Run("並行タスク", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				task := Task{
					Execute: func(ctx context.Context) (interface{}, error) {
						time.Sleep(time.Microsecond)
						return nil, nil
					},
				}
				_, _ = pool.Submit(context.Background(), task)
			}
		})
	})
}
