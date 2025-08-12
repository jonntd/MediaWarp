package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FolderCacheItem 文件夹缓存项
type FolderCacheItem struct {
	Folders   []string  `json:"folders"`
	Timestamp time.Time `json:"timestamp"`
	TTL       time.Duration `json:"ttl"`
}

// IsExpired 检查缓存是否过期
func (item *FolderCacheItem) IsExpired() bool {
	return time.Since(item.Timestamp) > item.TTL
}

// FolderCache 文件夹缓存管理器
type FolderCache struct {
	cache map[string]*FolderCacheItem
	mutex sync.RWMutex
	defaultTTL time.Duration
}

// NewFolderCache 创建新的文件夹缓存管理器
func NewFolderCache(defaultTTL time.Duration) *FolderCache {
	fc := &FolderCache{
		cache:      make(map[string]*FolderCacheItem),
		defaultTTL: defaultTTL,
	}
	
	// 启动清理协程
	go fc.startCleanupRoutine()
	
	return fc
}

// generateKey 生成缓存键
func (fc *FolderCache) generateKey(server, path string) string {
	if path == "" {
		path = "root"
	}
	return fmt.Sprintf("folders:%s:%s", server, path)
}

// Set 设置缓存
func (fc *FolderCache) Set(server, path string, folders []string) {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	
	key := fc.generateKey(server, path)
	fc.cache[key] = &FolderCacheItem{
		Folders:   folders,
		Timestamp: time.Now(),
		TTL:       fc.defaultTTL,
	}
}

// Get 获取缓存
func (fc *FolderCache) Get(server, path string) ([]string, bool) {
	fc.mutex.RLock()
	defer fc.mutex.RUnlock()
	
	key := fc.generateKey(server, path)
	item, exists := fc.cache[key]
	
	if !exists || item.IsExpired() {
		return nil, false
	}
	
	return item.Folders, true
}

// Delete 删除缓存
func (fc *FolderCache) Delete(server, path string) {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	
	key := fc.generateKey(server, path)
	delete(fc.cache, key)
}

// Clear 清除所有缓存
func (fc *FolderCache) Clear() {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	
	fc.cache = make(map[string]*FolderCacheItem)
}

// ClearByServer 清除指定服务器的所有缓存
func (fc *FolderCache) ClearByServer(server string) {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	
	prefix := fmt.Sprintf("folders:%s:", server)
	for key := range fc.cache {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(fc.cache, key)
		}
	}
}

// GetStats 获取缓存统计信息
func (fc *FolderCache) GetStats() map[string]interface{} {
	fc.mutex.RLock()
	defer fc.mutex.RUnlock()
	
	total := len(fc.cache)
	expired := 0
	
	for _, item := range fc.cache {
		if item.IsExpired() {
			expired++
		}
	}
	
	return map[string]interface{}{
		"total_items":   total,
		"expired_items": expired,
		"active_items":  total - expired,
		"default_ttl":   fc.defaultTTL.String(),
	}
}

// startCleanupRoutine 启动清理过期缓存的协程
func (fc *FolderCache) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
	defer ticker.Stop()
	
	for range ticker.C {
		fc.cleanupExpired()
	}
}

// cleanupExpired 清理过期的缓存项
func (fc *FolderCache) cleanupExpired() {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	
	for key, item := range fc.cache {
		if item.IsExpired() {
			delete(fc.cache, key)
		}
	}
}

// Export 导出缓存数据（用于调试）
func (fc *FolderCache) Export() ([]byte, error) {
	fc.mutex.RLock()
	defer fc.mutex.RUnlock()
	
	return json.MarshalIndent(fc.cache, "", "  ")
}

// 全局缓存实例
var GlobalFolderCache *FolderCache

// InitGlobalFolderCache 初始化全局文件夹缓存
func InitGlobalFolderCache(ttl time.Duration) {
	GlobalFolderCache = NewFolderCache(ttl)
}
