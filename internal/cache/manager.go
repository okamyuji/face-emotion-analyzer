package cache

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/okamyuji/face-emotion-analyzer/internal/errors"
)

// Item はキャッシュアイテムを表す
type Item struct {
	Value      interface{}
	Expiration time.Time
	Size       int64
}

// Stats はキャッシュの統計情報を表す
type Stats struct {
	ItemCount    int
	CurrentSize  int64
	MaxSize      int64
	UsagePercent float64
}

// Manager の定義を拡張
type Manager struct {
	mu              sync.RWMutex
	items           map[string]Item
	maxSize         int64
	currentSize     int64
	cleanupInterval time.Duration
	done            chan struct{}
}

// Config はキャッシュマネージャーの設定を表す
type Config struct {
	MaxSize         int64
	CleanupInterval time.Duration
}

// evict メソッドの実装を追加
func (m *Manager) evict(requiredSize int64) {
	type itemInfo struct {
		key        string
		expiration time.Time
		size       int64
	}

	items := make([]itemInfo, 0, len(m.items))
	for k, v := range m.items {
		items = append(items, itemInfo{
			key:        k,
			expiration: v.Expiration,
			size:       v.Size,
		})
	}

	// 有効期限が近い順にソート
	sort.Slice(items, func(i, j int) bool {
		return items[i].expiration.Before(items[j].expiration)
	})

	// 必要なサイズが確保できるまで削除
	for _, item := range items {
		if m.currentSize+requiredSize <= m.maxSize {
			break
		}
		if currentItem, exists := m.items[item.key]; exists {
			m.currentSize -= currentItem.Size
			delete(m.items, item.key)
		}
	}
}

// estimateSize の実装を追加
func estimateSize(value interface{}) int64 {
	if value == nil {
		return 0
	}

	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64, bool:
		return 8
	case map[string]interface{}:
		var size int64
		for k, val := range v {
			size += int64(len(k))
			size += estimateSize(val)
		}
		return size
	case []interface{}:
		var size int64
		for _, val := range v {
			size += estimateSize(val)
		}
		return size
	case struct{}:
		// 構造体のサイズを推定
		return 64 // デフォルトサイズ
	default:
		// その他の型のデフォルトサイズ
		return 32
	}
}

// GetStats メソッドを追加
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Stats{
		ItemCount:    len(m.items),
		CurrentSize:  m.currentSize,
		MaxSize:      m.maxSize,
		UsagePercent: float64(m.currentSize) / float64(m.maxSize) * 100,
	}
}

// NewManager は新しいキャッシュマネージャーを作成します
func NewManager(cfg Config) *Manager {
	m := &Manager{
		items:           make(map[string]Item),
		maxSize:         cfg.MaxSize,
		cleanupInterval: cfg.CleanupInterval,
		done:            make(chan struct{}),
	}

	go m.startCleanup()
	return m
}

// startCleanup は期限切れアイテムの定期的なクリーンアップを開始します
func (m *Manager) startCleanup() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup は期限切れのアイテムを削除します
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, item := range m.items {
		if item.Expiration.Before(now) {
			m.currentSize -= item.Size
			delete(m.items, key)
		}
	}
}

// Close はキャッシュマネージャーを停止します
func (m *Manager) Close() error {
	select {
	case <-m.done:
		// すでに閉じられている
		return nil
	default:
		close(m.done)
		return nil
	}
}

// Clear はキャッシュの内容をすべて削除します
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]Item)
	m.currentSize = 0
}

// isExpired は項目が期限切れかどうかを判定します
func (i Item) isExpired() bool {
	return time.Now().After(i.Expiration)
}

// Get はキーに対応する値を取得します
func (m *Manager) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[key]
	if !ok || item.isExpired() {
		return nil, errors.ErrKeyNotFound
	}
	return item.Value, nil
}

// Set はキーと値をキャッシュに設定します
func (m *Manager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	size := estimateSize(value)
	if size > m.maxSize {
		return errors.ErrSizeExceeded
	}

	if m.currentSize+size > m.maxSize {
		m.evict(size)
		if m.currentSize+size > m.maxSize {
			return errors.ErrSizeExceeded
		}
	}

	m.items[key] = Item{
		Value:      value,
		Expiration: time.Now().Add(ttl),
		Size:       size,
	}
	m.currentSize += size
	return nil
}

// GetOrCompute はキーに対応する値を取得し、存在しない場合は計算して設定します
func (m *Manager) GetOrCompute(ctx context.Context, key string, compute func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	value, err := m.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if err != errors.ErrKeyNotFound {
		return nil, err
	}

	value, err = compute()
	if err != nil {
		return nil, err
	}

	err = m.Set(ctx, key, value, ttl)
	if err != nil {
		return nil, err
	}

	return value, nil
}
