package cache

import (
	"MediaWarp/constants"
	"MediaWarp/internal/service/emby"
	"MediaWarp/internal/service/jellyfin"
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"
)

// PlaybackInfoCache 播放信息专用缓存管理器
type PlaybackInfoCache struct {
	itemInfoCache  map[string]*CachedItemInfo
	strmTypeCache  map[string]*CachedStrmType
	alistLinkCache map[string]*CachedAlistLink
	playbackCache  map[string]*CachedPlaybackInfo
	mutex          sync.RWMutex
	cleanup        *time.Ticker
	done           chan bool
	stats          *CacheStats
}

// CachedItemInfo 缓存的媒体项信息
type CachedItemInfo struct {
	EmbyItem     *emby.EmbyResponse `json:"emby_item,omitempty"`
	JellyfinItem *jellyfin.Response `json:"jellyfin_item,omitempty"`
	Timestamp    time.Time          `json:"timestamp"`
	TTL          time.Duration      `json:"ttl"`
}

// CachedStrmType 缓存的Strm文件类型
type CachedStrmType struct {
	Type      constants.StrmFileType `json:"type"`
	Option    any                    `json:"option"`
	Timestamp time.Time              `json:"timestamp"`
	TTL       time.Duration          `json:"ttl"`
}

// CachedAlistLink 缓存的Alist下载链接
type CachedAlistLink struct {
	DownloadURL string        `json:"download_url"`
	Sign        string        `json:"sign"`
	RawURL      string        `json:"raw_url"`
	Timestamp   time.Time     `json:"timestamp"`
	TTL         time.Duration `json:"ttl"`
}

// CachedPlaybackInfo 缓存的完整播放信息
type CachedPlaybackInfo struct {
	EmbyResponse     *emby.PlaybackInfoResponse     `json:"emby_response,omitempty"`
	JellyfinResponse *jellyfin.PlaybackInfoResponse `json:"jellyfin_response,omitempty"`
	Timestamp        time.Time                      `json:"timestamp"`
	TTL              time.Duration                  `json:"ttl"`
}

// CacheStats 缓存统计信息
type CacheStats struct {
	ItemInfoHits    int64 `json:"item_info_hits"`
	ItemInfoMisses  int64 `json:"item_info_misses"`
	StrmTypeHits    int64 `json:"strm_type_hits"`
	StrmTypeMisses  int64 `json:"strm_type_misses"`
	AlistLinkHits   int64 `json:"alist_link_hits"`
	AlistLinkMisses int64 `json:"alist_link_misses"`
	PlaybackHits    int64 `json:"playback_hits"`
	PlaybackMisses  int64 `json:"playback_misses"`
	TotalRequests   int64 `json:"total_requests"`
	mutex           sync.RWMutex
}

// NewPlaybackInfoCache 创建新的播放信息缓存管理器
func NewPlaybackInfoCache(cleanupInterval time.Duration) *PlaybackInfoCache {
	cache := &PlaybackInfoCache{
		itemInfoCache:  make(map[string]*CachedItemInfo),
		strmTypeCache:  make(map[string]*CachedStrmType),
		alistLinkCache: make(map[string]*CachedAlistLink),
		playbackCache:  make(map[string]*CachedPlaybackInfo),
		cleanup:        time.NewTicker(cleanupInterval),
		done:           make(chan bool),
		stats:          &CacheStats{},
	}

	// 启动清理协程
	go cache.startCleanup()

	return cache
}

// NewPlaybackInfoCacheWithConfig 使用配置创建缓存管理器
func NewPlaybackInfoCacheWithConfig() *PlaybackInfoCache {
	// 这里需要导入config包，暂时使用默认值
	// 后续可以通过依赖注入的方式传入配置
	return NewPlaybackInfoCache(5 * time.Minute)
}

// IsExpired 检查缓存项是否过期
func (c *CachedItemInfo) IsExpired() bool {
	return time.Now().After(c.Timestamp.Add(c.TTL))
}

func (c *CachedStrmType) IsExpired() bool {
	return time.Now().After(c.Timestamp.Add(c.TTL))
}

func (c *CachedAlistLink) IsExpired() bool {
	return time.Now().After(c.Timestamp.Add(c.TTL))
}

func (c *CachedPlaybackInfo) IsExpired() bool {
	return time.Now().After(c.Timestamp.Add(c.TTL))
}

// generateKey 生成缓存键
func (pic *PlaybackInfoCache) generateKey(prefix, identifier string) string {
	hash := md5.Sum([]byte(identifier))
	return prefix + ":" + hex.EncodeToString(hash[:])
}

// GetItemInfo 获取媒体项信息（消除重复调用的核心方法）
func (pic *PlaybackInfoCache) GetItemInfo(mediaSourceID string) (*CachedItemInfo, bool) {
	pic.mutex.RLock()
	defer pic.mutex.RUnlock()

	key := pic.generateKey("item", mediaSourceID)
	if cached, exists := pic.itemInfoCache[key]; exists && !cached.IsExpired() {
		pic.stats.incrementItemInfoHits()
		return cached, true
	}

	pic.stats.incrementItemInfoMisses()
	return nil, false
}

// SetItemInfo 设置媒体项信息缓存
func (pic *PlaybackInfoCache) SetItemInfo(mediaSourceID string, embyItem *emby.EmbyResponse, jellyfinItem *jellyfin.Response, ttl time.Duration) {
	pic.mutex.Lock()
	defer pic.mutex.Unlock()

	key := pic.generateKey("item", mediaSourceID)
	pic.itemInfoCache[key] = &CachedItemInfo{
		EmbyItem:     embyItem,
		JellyfinItem: jellyfinItem,
		Timestamp:    time.Now(),
		TTL:          ttl,
	}
}

// GetStrmType 获取Strm文件类型（消除重复解析）
func (pic *PlaybackInfoCache) GetStrmType(filePath string) (*CachedStrmType, bool) {
	pic.mutex.RLock()
	defer pic.mutex.RUnlock()

	key := pic.generateKey("strm", filePath)
	if cached, exists := pic.strmTypeCache[key]; exists && !cached.IsExpired() {
		pic.stats.incrementStrmTypeHits()
		return cached, true
	}

	pic.stats.incrementStrmTypeMisses()
	return nil, false
}

// SetStrmType 设置Strm文件类型缓存
func (pic *PlaybackInfoCache) SetStrmType(filePath string, strmType constants.StrmFileType, option any, ttl time.Duration) {
	pic.mutex.Lock()
	defer pic.mutex.Unlock()

	key := pic.generateKey("strm", filePath)
	pic.strmTypeCache[key] = &CachedStrmType{
		Type:      strmType,
		Option:    option,
		Timestamp: time.Now(),
		TTL:       ttl,
	}
}

// GetAlistLink 获取Alist下载链接
func (pic *PlaybackInfoCache) GetAlistLink(filePath string) (*CachedAlistLink, bool) {
	pic.mutex.RLock()
	defer pic.mutex.RUnlock()

	key := pic.generateKey("alist", filePath)
	if cached, exists := pic.alistLinkCache[key]; exists && !cached.IsExpired() {
		pic.stats.incrementAlistLinkHits()
		return cached, true
	}

	pic.stats.incrementAlistLinkMisses()
	return nil, false
}

// SetAlistLink 设置Alist下载链接缓存
func (pic *PlaybackInfoCache) SetAlistLink(filePath, downloadURL, sign, rawURL string, ttl time.Duration) {
	pic.mutex.Lock()
	defer pic.mutex.Unlock()

	key := pic.generateKey("alist", filePath)
	pic.alistLinkCache[key] = &CachedAlistLink{
		DownloadURL: downloadURL,
		Sign:        sign,
		RawURL:      rawURL,
		Timestamp:   time.Now(),
		TTL:         ttl,
	}
}

// GetPlaybackInfo 获取完整播放信息
func (pic *PlaybackInfoCache) GetPlaybackInfo(mediaSourceID string) (*CachedPlaybackInfo, bool) {
	pic.mutex.RLock()
	defer pic.mutex.RUnlock()

	key := pic.generateKey("playback", mediaSourceID)
	if cached, exists := pic.playbackCache[key]; exists && !cached.IsExpired() {
		pic.stats.incrementPlaybackHits()
		return cached, true
	}

	pic.stats.incrementPlaybackMisses()
	return nil, false
}

// SetPlaybackInfo 设置完整播放信息缓存
func (pic *PlaybackInfoCache) SetPlaybackInfo(mediaSourceID string, embyResponse *emby.PlaybackInfoResponse, jellyfinResponse *jellyfin.PlaybackInfoResponse, ttl time.Duration) {
	pic.mutex.Lock()
	defer pic.mutex.Unlock()

	key := pic.generateKey("playback", mediaSourceID)
	pic.playbackCache[key] = &CachedPlaybackInfo{
		EmbyResponse:     embyResponse,
		JellyfinResponse: jellyfinResponse,
		Timestamp:        time.Now(),
		TTL:              ttl,
	}
}

// startCleanup 启动清理过期项的协程
func (pic *PlaybackInfoCache) startCleanup() {
	for {
		select {
		case <-pic.cleanup.C:
			pic.cleanExpired()
		case <-pic.done:
			return
		}
	}
}

// cleanExpired 清理过期项
func (pic *PlaybackInfoCache) cleanExpired() {
	pic.mutex.Lock()
	defer pic.mutex.Unlock()

	// 清理过期的媒体项信息
	for key, item := range pic.itemInfoCache {
		if item.IsExpired() {
			delete(pic.itemInfoCache, key)
		}
	}

	// 清理过期的Strm类型信息
	for key, item := range pic.strmTypeCache {
		if item.IsExpired() {
			delete(pic.strmTypeCache, key)
		}
	}

	// 清理过期的Alist链接
	for key, item := range pic.alistLinkCache {
		if item.IsExpired() {
			delete(pic.alistLinkCache, key)
		}
	}

	// 清理过期的播放信息
	for key, item := range pic.playbackCache {
		if item.IsExpired() {
			delete(pic.playbackCache, key)
		}
	}
}

// GetStats 获取缓存统计信息
func (pic *PlaybackInfoCache) GetStats() CacheStats {
	pic.stats.mutex.RLock()
	defer pic.stats.mutex.RUnlock()

	// 创建副本以避免返回锁
	return CacheStats{
		ItemInfoHits:    pic.stats.ItemInfoHits,
		ItemInfoMisses:  pic.stats.ItemInfoMisses,
		StrmTypeHits:    pic.stats.StrmTypeHits,
		StrmTypeMisses:  pic.stats.StrmTypeMisses,
		AlistLinkHits:   pic.stats.AlistLinkHits,
		AlistLinkMisses: pic.stats.AlistLinkMisses,
		PlaybackHits:    pic.stats.PlaybackHits,
		PlaybackMisses:  pic.stats.PlaybackMisses,
		TotalRequests:   pic.stats.TotalRequests,
	}
}

// 统计方法
func (cs *CacheStats) incrementItemInfoHits() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.ItemInfoHits++
	cs.TotalRequests++
}

func (cs *CacheStats) incrementItemInfoMisses() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.ItemInfoMisses++
	cs.TotalRequests++
}

func (cs *CacheStats) incrementStrmTypeHits() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.StrmTypeHits++
}

func (cs *CacheStats) incrementStrmTypeMisses() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.StrmTypeMisses++
}

func (cs *CacheStats) incrementAlistLinkHits() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.AlistLinkHits++
}

func (cs *CacheStats) incrementAlistLinkMisses() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.AlistLinkMisses++
}

func (cs *CacheStats) incrementPlaybackHits() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.PlaybackHits++
}

func (cs *CacheStats) incrementPlaybackMisses() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.PlaybackMisses++
}

// Close 关闭缓存管理器
func (pic *PlaybackInfoCache) Close() {
	pic.cleanup.Stop()
	close(pic.done)
}

// 全局播放信息缓存实例
var GlobalPlaybackCache = NewPlaybackInfoCache(5 * time.Minute)
