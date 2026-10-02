// Package kvstore 实现一个并发安全的内存键值存储，支持自动过期与 JSON 持久化。
package kvstore

import (
	"errors"
	"sync"
	"time"
)

// 预定义错误，调用方可用 errors.Is 判断错误类型。
var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExpired  = errors.New("key expired")
)

// cleanupInterval 过期扫描周期。
const cleanupInterval = time.Second

// Item 单个键值对条目，ExpireAt 为零值表示永不过期。
type Item struct {
	Value    string    `json:"value"`
	ExpireAt time.Time `json:"expire_at"`
}

// expired 判断条目在时刻 t 是否已过期。
func (it Item) expired(t time.Time) bool {
	return !it.ExpireAt.IsZero() && t.After(it.ExpireAt)
}

// KVStore 内存 KV 存储核心结构体。
type KVStore struct {
	data     map[string]Item
	mu       sync.RWMutex
	stopChan chan struct{}
	wg       sync.WaitGroup
	savePath string
}

// NewKVStore 创建存储实例：加载历史数据并启动后台过期清理协程。
func NewKVStore(savePath string) *KVStore {
	s := &KVStore{
		data:     make(map[string]Item),
		stopChan: make(chan struct{}),
		savePath: savePath,
	}

	s.load()

	s.wg.Add(1)
	go s.startCleanup()

	return s
}

// Set 写入键值对，ttl <= 0 表示永不过期；重复写入会覆盖旧值与旧过期时间。
func (s *KVStore) Set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := Item{Value: value}
	if ttl > 0 {
		item.ExpireAt = time.Now().Add(ttl)
	}
	s.data[key] = item
}

// Get 读取键对应的值；键不存在返回 ErrKeyNotFound，已过期返回 ErrKeyExpired。
func (s *KVStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.data[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	if item.expired(time.Now()) {
		return "", ErrKeyExpired
	}
	return item.Value, nil
}

// Del 删除指定键，键不存在时静默返回。
func (s *KVStore) Del(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Keys 返回当前未过期的所有键，结果已排序。
func (s *KVStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(s.data))
	for k, item := range s.data {
		if !item.expired(now) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Len 返回当前未过期的键数量。
func (s *KVStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	n := 0
	for _, item := range s.data {
		if !item.expired(now) {
			n++
		}
	}
	return n
}

// startCleanup 后台协程：按固定周期扫描并删除过期键。
func (s *KVStore) startCleanup() {
	defer s.wg.Done()

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanExpired()
		case <-s.stopChan:
			return
		}
	}
}

// cleanExpired 删除所有已过期的键。
func (s *KVStore) cleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for k, item := range s.data {
		if item.expired(now) {
			delete(s.data, k)
		}
	}
}

// Close 优雅关闭：通知后台协程退出，等待其结束后持久化数据。
func (s *KVStore) Close() {
	close(s.stopChan)
	s.wg.Wait()
	_ = s.Save()
}
