package kvstore

import (
	"path/filepath"
	"testing"
	"time"
)

// newTestStore 在临时目录中创建存储，测试结束自动清理持久化文件。
func newTestStore(t *testing.T) *KVStore {
	t.Helper()
	store := NewKVStore(filepath.Join(t.TempDir(), "data.json"))
	t.Cleanup(store.Close)
	return store
}

// TestSetGet 表驱动测试基础读写。
func TestSetGet(t *testing.T) {
	store := newTestStore(t)

	testCases := []struct {
		key   string
		value string
	}{
		{"name", "Tom"},
		{"age", "25"},
		{"email", "tom@example.com"},
	}

	for _, tc := range testCases {
		store.Set(tc.key, tc.value, 0)
		got, err := store.Get(tc.key)
		if err != nil {
			t.Errorf("Get(%s) 失败: %v", tc.key, err)
			continue
		}
		if got != tc.value {
			t.Errorf("Get(%s) = %q, 期望 %q", tc.key, got, tc.value)
		}
	}

	if n := store.Len(); n != len(testCases) {
		t.Errorf("Len() = %d, 期望 %d", n, len(testCases))
	}
}

// TestNotFound 测试读取不存在的键。
func TestNotFound(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Get("missing"); err != ErrKeyNotFound {
		t.Errorf("Get(missing) err = %v, 期望 %v", err, ErrKeyNotFound)
	}
}

// TestExpire 测试 TTL 过期。
func TestExpire(t *testing.T) {
	store := newTestStore(t)

	store.Set("key1", "val1", 100*time.Millisecond)
	if _, err := store.Get("key1"); err != nil {
		t.Fatalf("刚写入的 key 应存在, err = %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if _, err := store.Get("key1"); err != ErrKeyExpired {
		t.Errorf("过期后 err = %v, 期望 %v", err, ErrKeyExpired)
	}
}

// TestCleanup 测试后台协程会真正删除过期键。
func TestCleanup(t *testing.T) {
	store := newTestStore(t)

	store.Set("key1", "val1", 100*time.Millisecond)
	time.Sleep(1500 * time.Millisecond)

	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.data["key1"]; ok {
		t.Error("过期键应已被后台协程清理")
	}
}

// TestDel 测试删除。
func TestDel(t *testing.T) {
	store := newTestStore(t)

	store.Set("key1", "val1", 0)
	store.Del("key1")

	if _, err := store.Get("key1"); err != ErrKeyNotFound {
		t.Errorf("删除后 err = %v, 期望 %v", err, ErrKeyNotFound)
	}
}

// TestKeys 测试键列表（不含过期键）。
func TestKeys(t *testing.T) {
	store := newTestStore(t)

	store.Set("a", "1", 0)
	store.Set("b", "2", 0)
	store.Set("c", "3", 100*time.Millisecond)

	if got := len(store.Keys()); got != 3 {
		t.Errorf("未过期时 Keys() 数量 = %d, 期望 3", got)
	}

	time.Sleep(200 * time.Millisecond)
	if got := len(store.Keys()); got != 2 {
		t.Errorf("过期后 Keys() 数量 = %d, 期望 2", got)
	}
}

// TestPersist 测试重启后数据可恢复，且过期数据不落盘。
func TestPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	s1 := NewKVStore(path)
	s1.Set("keep", "yes", 0)
	s1.Set("gone", "no", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	s1.Close()

	s2 := NewKVStore(path)
	defer s2.Close()

	got, err := s2.Get("keep")
	if err != nil || got != "yes" {
		t.Errorf("重启后 Get(keep) = %q, %v; 期望 \"yes\", nil", got, err)
	}
	if _, err := s2.Get("gone"); err != ErrKeyNotFound {
		t.Errorf("过期键不应被加载, err = %v", err)
	}
}

// TestConcurrent 并发读写压力测试（配合 -race 运行）。
func TestConcurrent(t *testing.T) {
	store := newTestStore(t)

	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 200; j++ {
				key := string(rune('a'+n)) + string(rune('0'+j%10))
				store.Set(key, "v", 0)
				store.Get(key)
				store.Keys()
				store.Del(key)
			}
		}(i)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
