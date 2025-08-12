package cache

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	URL        string
	ExpireTime time.Time
}

// SafeCache 线程安全的缓存管理器
type SafeCache struct {
	data    map[string]CacheItem
	mutex   sync.RWMutex
	cleanup *time.Ticker
	done    chan bool
}

// NewSafeCache 创建新的安全缓存
func NewSafeCache(cleanupInterval time.Duration) *SafeCache {
	cache := &SafeCache{
		data:    make(map[string]CacheItem),
		cleanup: time.NewTicker(cleanupInterval),
		done:    make(chan bool),
	}
	
	// 启动清理协程
	go cache.startCleanup()
	
	return cache
}

// Get 获取缓存项
func (c *SafeCache) Get(key string) (CacheItem, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return CacheItem{}, false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpireTime) {
		// 在读锁中不能删除，标记为不存在
		return CacheItem{}, false
	}
	
	return item, true
}

// Set 设置缓存项
func (c *SafeCache) Set(key string, url string, expireTime time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data[key] = CacheItem{
		URL:        url,
		ExpireTime: expireTime,
	}
}

// Delete 删除缓存项
func (c *SafeCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.data, key)
}

// Size 获取缓存大小
func (c *SafeCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return len(c.data)
}

// Clear 清空缓存
func (c *SafeCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data = make(map[string]CacheItem)
}

// startCleanup 启动清理过期项的协程
func (c *SafeCache) startCleanup() {
	for {
		select {
		case <-c.cleanup.C:
			c.cleanExpired()
		case <-c.done:
			return
		}
	}
}

// cleanExpired 清理过期项
func (c *SafeCache) cleanExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	for key, item := range c.data {
		if now.After(item.ExpireTime) {
			delete(c.data, key)
		}
	}
}

// Close 关闭缓存管理器
func (c *SafeCache) Close() {
	c.cleanup.Stop()
	close(c.done)
}

// Stats 获取缓存统计信息
func (c *SafeCache) Stats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	now := time.Now()
	expired := 0
	for _, item := range c.data {
		if now.After(item.ExpireTime) {
			expired++
		}
	}
	
	return map[string]interface{}{
		"total":   len(c.data),
		"expired": expired,
		"active":  len(c.data) - expired,
	}
}
