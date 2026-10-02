package kvstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Save 将未过期的数据以格式化 JSON 写入持久化文件（先写临时文件再重命名，避免中途失败留下残缺文件）。
func (s *KVStore) Save() error {
	s.mu.RLock()
	now := time.Now()
	snapshot := make(map[string]Item, len(s.data))
	for k, item := range s.data {
		if !item.expired(now) {
			snapshot[k] = item
		}
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.savePath)
	tmp, err := os.CreateTemp(dir, ".kv-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, s.savePath)
}

// load 启动时加载历史数据；文件不存在视为首次运行，文件损坏视为致命错误。
func (s *KVStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if len(data) == 0 {
		return
	}

	// 先反序列化到临时 map，过滤掉历史文件中的过期条目。
	loaded := make(map[string]Item)
	if err = json.Unmarshal(data, &loaded); err != nil {
		panic("load " + s.savePath + " failed: " + err.Error())
	}
	now := time.Now()
	for k, item := range loaded {
		if !item.expired(now) {
			s.data[k] = item
		}
	}
}
