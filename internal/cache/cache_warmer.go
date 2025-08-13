package cache

import (
	"MediaWarp/internal/logging"
	"MediaWarp/internal/service/emby"
	"MediaWarp/internal/service/jellyfin"
	"sync"
	"time"
)

// CacheWarmer 缓存预热器
type CacheWarmer struct {
	cache          *PlaybackInfoCache
	embyServer     *emby.EmbyServer
	jellyfinServer *jellyfin.Jellyfin
	serverType     string

	// 预热配置
	config WarmupConfig

	// 热门内容列表
	popularItems []string
	recentItems  []string
	mutex        sync.RWMutex

	// 预热统计
	stats WarmupStats
}

// WarmupConfig 预热配置
type WarmupConfig struct {
	Enabled               bool          `json:"enabled"`
	StartupWarmupEnabled  bool          `json:"startup_warmup_enabled"`
	OnAccessWarmupEnabled bool          `json:"on_access_warmup_enabled"`
	MaxPopularItems       int           `json:"max_popular_items"`
	MaxRecentItems        int           `json:"max_recent_items"`
	WarmupInterval        time.Duration `json:"warmup_interval"`
	WarmupTimeout         time.Duration `json:"warmup_timeout"`
	RelatedItemsCount     int           `json:"related_items_count"`
}

// WarmupStats 预热统计
type WarmupStats struct {
	TotalWarmupRequests   int64         `json:"total_warmup_requests"`
	SuccessfulWarmups     int64         `json:"successful_warmups"`
	FailedWarmups         int64         `json:"failed_warmups"`
	LastWarmupTime        time.Time     `json:"last_warmup_time"`
	AverageWarmupDuration time.Duration `json:"average_warmup_duration"`
	mutex                 sync.RWMutex
}

// NewCacheWarmer 创建缓存预热器
func NewCacheWarmer(cache *PlaybackInfoCache, serverType string) *CacheWarmer {
	return &CacheWarmer{
		cache:      cache,
		serverType: serverType,
		config: WarmupConfig{
			Enabled:               true,
			StartupWarmupEnabled:  true,
			OnAccessWarmupEnabled: true,
			MaxPopularItems:       100,
			MaxRecentItems:        50,
			WarmupInterval:        5 * time.Minute,
			WarmupTimeout:         30 * time.Second,
			RelatedItemsCount:     5,
		},
		popularItems: make([]string, 0),
		recentItems:  make([]string, 0),
		stats:        WarmupStats{},
	}
}

// StartupWarmup 启动时预热
func (cw *CacheWarmer) StartupWarmup() {
	if !cw.config.Enabled || !cw.config.StartupWarmupEnabled {
		logging.Info("启动预热已禁用")
		return
	}

	logging.Info("开始启动预热...")
	startTime := time.Now()

	// 异步执行预热，避免阻塞启动
	go func() {
		defer func() {
			duration := time.Since(startTime)
			logging.Info("启动预热完成，耗时：", duration)
			cw.updateWarmupStats(true, duration)
		}()

		// 预热热门内容
		cw.warmupPopularItems()

		// 预热最近访问的内容
		cw.warmupRecentItems()
	}()
}

// OnAccessWarmup 访问时预热相关内容
func (cw *CacheWarmer) OnAccessWarmup(itemID string) {
	if !cw.config.Enabled || !cw.config.OnAccessWarmupEnabled {
		return
	}

	logging.Debug("触发访问预热：", itemID)

	// 异步预热，不阻塞当前请求
	go func() {
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			cw.updateWarmupStats(true, duration)
		}()

		// 添加到最近访问列表
		cw.addToRecentItems(itemID)

		// 预热相关内容
		cw.warmupRelatedItems(itemID)
	}()
}

// warmupPopularItems 预热热门内容
func (cw *CacheWarmer) warmupPopularItems() {
	cw.mutex.RLock()
	items := make([]string, len(cw.popularItems))
	copy(items, cw.popularItems)
	cw.mutex.RUnlock()

	logging.Info("预热热门内容，数量：", len(items))

	for _, itemID := range items {
		if err := cw.warmupSingleItem(itemID); err != nil {
			logging.Warning("预热热门内容失败：", itemID, "错误：", err)
			cw.updateWarmupStats(false, 0)
		}
	}
}

// warmupRecentItems 预热最近访问的内容
func (cw *CacheWarmer) warmupRecentItems() {
	cw.mutex.RLock()
	items := make([]string, len(cw.recentItems))
	copy(items, cw.recentItems)
	cw.mutex.RUnlock()

	logging.Info("预热最近内容，数量：", len(items))

	for _, itemID := range items {
		if err := cw.warmupSingleItem(itemID); err != nil {
			logging.Warning("预热最近内容失败：", itemID, "错误：", err)
			cw.updateWarmupStats(false, 0)
		}
	}
}

// warmupRelatedItems 预热相关内容
func (cw *CacheWarmer) warmupRelatedItems(itemID string) {
	// 这里可以实现更复杂的相关内容发现逻辑
	// 例如：同一系列的其他集数、相同类型的内容等

	logging.Debug("预热相关内容：", itemID)

	// 简单实现：预热相邻的几个ID
	relatedItems := cw.generateRelatedItemIDs(itemID)

	for _, relatedID := range relatedItems {
		if err := cw.warmupSingleItem(relatedID); err != nil {
			logging.Debug("预热相关内容失败：", relatedID, "错误：", err)
		}
	}
}

// warmupSingleItem 预热单个内容项
func (cw *CacheWarmer) warmupSingleItem(itemID string) error {
	// 检查是否已经缓存
	if _, found := cw.cache.GetItemInfo(itemID); found {
		logging.Debug("内容已缓存，跳过预热：", itemID)
		return nil
	}

	logging.Debug("开始预热内容：", itemID)

	// 获取媒体项信息
	var itemResponse interface{}
	var err error

	switch cw.serverType {
	case "emby":
		if cw.embyServer != nil {
			itemResponse, err = cw.embyServer.ItemsServiceQueryItem(itemID, 1, "Path,MediaSources")
			if err == nil {
				cw.cache.SetItemInfo(itemID, itemResponse.(*emby.EmbyResponse), nil, 30*time.Minute)
			}
		}
	case "jellyfin":
		if cw.jellyfinServer != nil {
			itemResponse, err = cw.jellyfinServer.ItemsServiceQueryItem(itemID, 1, "Path,MediaSources")
			if err == nil {
				cw.cache.SetItemInfo(itemID, nil, itemResponse.(*jellyfin.Response), 30*time.Minute)
			}
		}
	}

	if err != nil {
		return err
	}

	// 如果是Strm文件，预热文件类型信息
	if itemResponse != nil {
		path := cw.extractItemPath(itemResponse)
		if path != "" && cw.isStrmFile(path) {
			// 注意：这里暂时跳过Strm类型识别，避免循环导入
			// 实际使用时需要通过依赖注入的方式传入识别函数
			logging.Debug("发现Strm文件，路径：", path)
		}
	}

	logging.Debug("预热完成：", itemID)
	return nil
}

// addToRecentItems 添加到最近访问列表
func (cw *CacheWarmer) addToRecentItems(itemID string) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	// 检查是否已存在
	for i, id := range cw.recentItems {
		if id == itemID {
			// 移动到最前面
			cw.recentItems = append([]string{itemID}, append(cw.recentItems[:i], cw.recentItems[i+1:]...)...)
			return
		}
	}

	// 添加到最前面
	cw.recentItems = append([]string{itemID}, cw.recentItems...)

	// 限制列表大小
	if len(cw.recentItems) > cw.config.MaxRecentItems {
		cw.recentItems = cw.recentItems[:cw.config.MaxRecentItems]
	}
}

// generateRelatedItemIDs 生成相关内容ID
func (cw *CacheWarmer) generateRelatedItemIDs(itemID string) []string {
	// 简单实现：生成相邻的几个ID
	// 实际应该基于更复杂的逻辑，如系列信息、类型等

	var relatedIDs []string

	// 尝试解析数字ID并生成相邻ID
	// 这是一个简化的实现
	for i := 1; i <= cw.config.RelatedItemsCount; i++ {
		relatedIDs = append(relatedIDs, itemID+"-related-"+string(rune(i)))
	}

	return relatedIDs
}

// extractItemPath 提取内容路径
func (cw *CacheWarmer) extractItemPath(itemResponse interface{}) string {
	switch cw.serverType {
	case "emby":
		if embyItem, ok := itemResponse.(*emby.EmbyResponse); ok && len(embyItem.Items) > 0 {
			if embyItem.Items[0].Path != nil {
				return *embyItem.Items[0].Path
			}
		}
	case "jellyfin":
		if jellyfinItem, ok := itemResponse.(*jellyfin.Response); ok && len(jellyfinItem.Items) > 0 {
			if jellyfinItem.Items[0].Path != nil {
				return *jellyfinItem.Items[0].Path
			}
		}
	}
	return ""
}

// isStrmFile 检查是否为Strm文件
func (cw *CacheWarmer) isStrmFile(path string) bool {
	return len(path) > 5 && path[len(path)-5:] == ".strm"
}

// updateWarmupStats 更新预热统计
func (cw *CacheWarmer) updateWarmupStats(success bool, duration time.Duration) {
	cw.stats.mutex.Lock()
	defer cw.stats.mutex.Unlock()

	cw.stats.TotalWarmupRequests++
	cw.stats.LastWarmupTime = time.Now()

	if success {
		cw.stats.SuccessfulWarmups++
		// 更新平均预热时间
		if cw.stats.SuccessfulWarmups == 1 {
			cw.stats.AverageWarmupDuration = duration
		} else {
			cw.stats.AverageWarmupDuration = (cw.stats.AverageWarmupDuration*time.Duration(cw.stats.SuccessfulWarmups-1) + duration) / time.Duration(cw.stats.SuccessfulWarmups)
		}
	} else {
		cw.stats.FailedWarmups++
	}
}

// GetWarmupStats 获取预热统计
func (cw *CacheWarmer) GetWarmupStats() WarmupStats {
	cw.stats.mutex.RLock()
	defer cw.stats.mutex.RUnlock()

	return WarmupStats{
		TotalWarmupRequests:   cw.stats.TotalWarmupRequests,
		SuccessfulWarmups:     cw.stats.SuccessfulWarmups,
		FailedWarmups:         cw.stats.FailedWarmups,
		LastWarmupTime:        cw.stats.LastWarmupTime,
		AverageWarmupDuration: cw.stats.AverageWarmupDuration,
	}
}

// SetPopularItems 设置热门内容列表
func (cw *CacheWarmer) SetPopularItems(items []string) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	if len(items) > cw.config.MaxPopularItems {
		items = items[:cw.config.MaxPopularItems]
	}

	cw.popularItems = make([]string, len(items))
	copy(cw.popularItems, items)

	logging.Info("更新热门内容列表，数量：", len(cw.popularItems))
}

// StartPeriodicWarmup 启动定期预热
func (cw *CacheWarmer) StartPeriodicWarmup() {
	if !cw.config.Enabled {
		return
	}

	logging.Info("启动定期预热，间隔：", cw.config.WarmupInterval)

	go func() {
		ticker := time.NewTicker(cw.config.WarmupInterval)
		defer ticker.Stop()

		for range ticker.C {
			logging.Debug("执行定期预热")
			cw.warmupPopularItems()
		}
	}()
}

// 全局缓存预热器实例
var GlobalCacheWarmer *CacheWarmer

// InitGlobalCacheWarmer 初始化全局缓存预热器
func InitGlobalCacheWarmer(serverType string) {
	GlobalCacheWarmer = NewCacheWarmer(GlobalPlaybackCache, serverType)
}
