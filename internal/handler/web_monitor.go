package handler

import (
	"MediaWarp/internal/cache"
	"MediaWarp/internal/logging"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// WebMonitorHandler Web监控处理器
type WebMonitorHandler struct {
	cache     *cache.PlaybackInfoCache
	startTime time.Time
}

// NewWebMonitorHandler 创建Web监控处理器
func NewWebMonitorHandler() *WebMonitorHandler {
	return &WebMonitorHandler{
		cache:     cache.GlobalPlaybackCache,
		startTime: time.Now(),
	}
}

// GetMonitorData 获取实时监控数据
func (h *WebMonitorHandler) GetMonitorData(ctx *gin.Context) {
	logging.Debug("======= GetMonitorData =======")

	// 获取缓存统计
	cacheStats := h.cache.GetStats()

	// 简化的响应数据
	response := gin.H{
		"timestamp": time.Now(),
		"cache_stats": gin.H{
			"total_requests":   cacheStats.TotalRequests,
			"item_info_hits":   cacheStats.ItemInfoHits,
			"item_info_misses": cacheStats.ItemInfoMisses,
			"strm_type_hits":   cacheStats.StrmTypeHits,
			"strm_type_misses": cacheStats.StrmTypeMisses,
		},
		"hit_rates": gin.H{
			"overall_hit_rate": h.calculateOverallHitRate(&cacheStats),
		},
		"system_stats":        h.getSystemStats(),
		"warmup_stats":        h.getWarmupStats(),
		"deduplication_stats": h.getDeduplicationStats(),
	}

	ctx.JSON(http.StatusOK, response)
}

// calculateOverallHitRate 计算总体命中率
func (h *WebMonitorHandler) calculateOverallHitRate(stats *cache.CacheStats) float64 {
	totalRequests := stats.ItemInfoHits + stats.ItemInfoMisses + stats.StrmTypeHits + stats.StrmTypeMisses
	if totalRequests == 0 {
		return 0
	}
	totalHits := stats.ItemInfoHits + stats.StrmTypeHits
	return float64(totalHits) / float64(totalRequests) * 100
}

// getSystemStats 获取系统统计
func (h *WebMonitorHandler) getSystemStats() gin.H {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(h.startTime).Seconds()

	return gin.H{
		"uptime_seconds":  int64(uptime),
		"memory_usage_mb": float64(memStats.Alloc) / 1024 / 1024,
		"goroutine_count": runtime.NumGoroutine(),
		"gc_count":        memStats.NumGC,
	}
}

// getWarmupStats 获取预热统计
func (h *WebMonitorHandler) getWarmupStats() gin.H {
	if cache.GlobalCacheWarmer == nil {
		return gin.H{
			"enabled": false,
		}
	}

	stats := cache.GlobalCacheWarmer.GetWarmupStats()

	var successRate float64
	if stats.TotalWarmupRequests > 0 {
		successRate = float64(stats.SuccessfulWarmups) / float64(stats.TotalWarmupRequests) * 100
	}

	return gin.H{
		"enabled":                    true,
		"total_warmup_requests":      stats.TotalWarmupRequests,
		"successful_warmups":         stats.SuccessfulWarmups,
		"failed_warmups":             stats.FailedWarmups,
		"success_rate":               successRate,
		"average_warmup_duration_ms": stats.AverageWarmupDuration.Milliseconds(),
	}
}

// calculateHitRates 计算各种缓存的命中率
func (h *WebMonitorHandler) calculateHitRates(stats *cache.CacheStats) HitRates {
	var itemInfoHitRate, strmTypeHitRate, alistLinkHitRate, playbackHitRate, overallHitRate float64

	// 计算媒体项信息命中率
	itemInfoTotal := stats.ItemInfoHits + stats.ItemInfoMisses
	if itemInfoTotal > 0 {
		itemInfoHitRate = float64(stats.ItemInfoHits) / float64(itemInfoTotal) * 100
	}

	// 计算Strm类型命中率
	strmTypeTotal := stats.StrmTypeHits + stats.StrmTypeMisses
	if strmTypeTotal > 0 {
		strmTypeHitRate = float64(stats.StrmTypeHits) / float64(strmTypeTotal) * 100
	}

	// 计算Alist链接命中率
	alistLinkTotal := stats.AlistLinkHits + stats.AlistLinkMisses
	if alistLinkTotal > 0 {
		alistLinkHitRate = float64(stats.AlistLinkHits) / float64(alistLinkTotal) * 100
	}

	// 计算播放信息命中率
	playbackTotal := stats.PlaybackHits + stats.PlaybackMisses
	if playbackTotal > 0 {
		playbackHitRate = float64(stats.PlaybackHits) / float64(playbackTotal) * 100
	}

	// 计算总体命中率
	totalRequests := itemInfoTotal + strmTypeTotal + alistLinkTotal + playbackTotal
	if totalRequests > 0 {
		totalHits := stats.ItemInfoHits + stats.StrmTypeHits + stats.AlistLinkHits + stats.PlaybackHits
		overallHitRate = float64(totalHits) / float64(totalRequests) * 100
	}

	return HitRates{
		ItemInfoHitRate:  itemInfoHitRate,
		StrmTypeHitRate:  strmTypeHitRate,
		AlistLinkHitRate: alistLinkHitRate,
		PlaybackHitRate:  playbackHitRate,
		OverallHitRate:   overallHitRate,
	}
}

// ServeMonitorPage 提供监控页面
func (h *WebMonitorHandler) ServeMonitorPage(ctx *gin.Context) {
	ctx.File("static/monitor.html")
}

// RegisterWebMonitorRoutes 注册Web监控路由
func RegisterWebMonitorRoutes(router *gin.Engine) {
	handler := NewWebMonitorHandler()

	// 静态文件服务
	router.Static("/static", "static")

	// Web监控页面
	router.GET("/monitor", handler.ServeMonitorPage)
	router.GET("/test", func(ctx *gin.Context) {
		ctx.File("static/test.html")
	})

	// 监控数据API
	api := router.Group("/api/monitor")
	{
		api.GET("/data", handler.GetMonitorData)
	}

	logging.Info("Web监控系统已启用，访问地址: /monitor")
}

// getDeduplicationStats 获取请求去重统计
func (h *WebMonitorHandler) getDeduplicationStats() gin.H {
	if cache.GlobalRequestDeduplicator == nil {
		return gin.H{
			"enabled": false,
		}
	}

	stats := cache.GlobalRequestDeduplicator.GetStats()

	var deduplicationRate float64
	if stats.TotalRequests > 0 {
		deduplicationRate = float64(stats.DeduplicatedCount) / float64(stats.TotalRequests) * 100
	}

	return gin.H{
		"enabled":              true,
		"total_requests":       stats.TotalRequests,
		"deduplicated_count":   stats.DeduplicatedCount,
		"deduplication_rate":   deduplicationRate,
		"saved_time_ms":        stats.SavedTime.Milliseconds(),
		"average_wait_time_ms": stats.AverageWaitTime.Milliseconds(),
	}
}
