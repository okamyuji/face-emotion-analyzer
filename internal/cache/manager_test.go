package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/okamyuji/face-emotion-analyzer/internal/errors"
)

func TestCacheManager(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{
			name: "基本的なセットと取得",
			fn: func(t *testing.T) {
				cfg := Config{
					MaxSize:         1024 * 1024,
					CleanupInterval: 100 * time.Millisecond,
				}
				m := NewManager(cfg)
				defer m.Close()

				ctx := context.Background()
				key := "test-key"
				value := []byte("test-value")
				ttl := 500 * time.Millisecond

				err := m.Set(ctx, key, value, ttl)
				assert.NoError(t, err)

				got, err := m.Get(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, value, got)
			},
		},
		{
			name: "期限切れ",
			fn: func(t *testing.T) {
				cfg := Config{
					MaxSize:         1024 * 1024,
					CleanupInterval: 100 * time.Millisecond,
				}
				m := NewManager(cfg)
				defer m.Close()

				ctx := context.Background()
				key := "test-key"
				value := []byte("test-value")
				ttl := 200 * time.Millisecond

				err := m.Set(ctx, key, value, ttl)
				assert.NoError(t, err)

				time.Sleep(300 * time.Millisecond)

				_, err = m.Get(ctx, key)
				assert.Error(t, err)
				assert.Equal(t, errors.ErrKeyNotFound, err)
			},
		},
		{
			name: "容量制限",
			fn: func(t *testing.T) {
				cfg := Config{
					MaxSize:         10,
					CleanupInterval: 100 * time.Millisecond,
				}
				m := NewManager(cfg)
				defer m.Close()

				ctx := context.Background()
				key := "test-key"
				value := []byte("test-value-long")
				ttl := 1 * time.Second

				err := m.Set(ctx, key, value, ttl)
				assert.Error(t, err)
				assert.Equal(t, errors.ErrSizeExceeded, err)
			},
		},
		{
			name: "並行アクセス",
			fn: func(t *testing.T) {
				cfg := Config{
					MaxSize:         1024 * 1024,
					CleanupInterval: 100 * time.Millisecond,
				}
				m := NewManager(cfg)
				defer m.Close()

				ctx := context.Background()
				var wg sync.WaitGroup
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						for j := 0; j < 10; j++ {
							key := fmt.Sprintf("key-%d-%d", id, j)
							value := []byte(fmt.Sprintf("value-%d-%d", id, j))
							err := m.Set(ctx, key, value, 500*time.Millisecond)
							assert.NoError(t, err)

							got, err := m.Get(ctx, key)
							assert.NoError(t, err)
							assert.Equal(t, value, got)
						}
					}(i)
				}
				wg.Wait()
			},
		},
		{
			name: "GetOrCompute",
			fn: func(t *testing.T) {
				cfg := Config{
					MaxSize:         1024 * 1024,
					CleanupInterval: 100 * time.Millisecond,
				}
				m := NewManager(cfg)
				defer m.Close()

				ctx := context.Background()
				key := "test-key"
				value := []byte("computed-value")
				ttl := 500 * time.Millisecond
				computeCalled := 0

				compute := func() (interface{}, error) {
					computeCalled++
					return value, nil
				}

				// 最初の呼び出し
				got, err := m.GetOrCompute(ctx, key, compute, ttl)
				assert.NoError(t, err)
				assert.Equal(t, value, got)
				assert.Equal(t, 1, computeCalled)

				// キャッシュからの取得
				got, err = m.GetOrCompute(ctx, key, compute, ttl)
				assert.NoError(t, err)
				assert.Equal(t, value, got)
				assert.Equal(t, 1, computeCalled)

				// 期限切れ後の再計算
				time.Sleep(600 * time.Millisecond)
				got, err = m.GetOrCompute(ctx, key, compute, ttl)
				assert.NoError(t, err)
				assert.Equal(t, value, got)
				assert.Equal(t, 2, computeCalled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestCacheEviction(t *testing.T) {
	manager := NewManager(Config{
		MaxSize:         1000,
		CleanupInterval: 100 * time.Millisecond,
	})
	defer manager.Close()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := make([]byte, 90)
		err := manager.Set(ctx, key, value, time.Minute)
		if err != nil {
			t.Errorf("Set() error = %v", err)
		}
	}

	newKey := "new-key"
	newValue := make([]byte, 100)
	err := manager.Set(ctx, newKey, newValue, time.Minute)
	if err != nil {
		t.Errorf("Set() error = %v", err)
	}

	stats := manager.GetStats()
	if stats.CurrentSize > manager.maxSize {
		t.Errorf("CurrentSize = %d, want <= %d", stats.CurrentSize, manager.maxSize)
	}
}

func BenchmarkCacheOperations(b *testing.B) {
	manager := NewManager(Config{
		MaxSize:         1024 * 1024, // 1MB
		CleanupInterval: time.Hour,   // クリーンアップの頻度を下げる
	})
	defer manager.Close()

	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		b.StopTimer()
		const valueSize = 64 // より小さいサイズを使用
		value := make([]byte, valueSize)
		key := "test-key"
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			_ = manager.Set(ctx, key, value, time.Minute)
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.StopTimer()
		const valueSize = 64
		key := "bench-key"
		value := make([]byte, valueSize)
		_ = manager.Set(ctx, key, value, time.Hour)
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			_, _ = manager.Get(ctx, key)
		}
	})

	b.Run("GetOrCompute", func(b *testing.B) {
		b.StopTimer()
		const valueSize = 64
		computedValue := make([]byte, valueSize)
		compute := func() (interface{}, error) {
			return computedValue, nil
		}
		key := "compute-key"
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			_, _ = manager.GetOrCompute(ctx, key, compute, time.Minute)
		}
	})
}
