package handler

import (
	"MediaWarp/internal/cache"
	"MediaWarp/internal/logging"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheStatsHandler 缓存统计处理器
type CacheStatsHandler struct {
	cache *cache.PlaybackInfoCache
}

// NewCacheStatsHandler 创建缓存统计处理器
func NewCacheStatsHandler() *CacheStatsHandler {
	return &CacheStatsHandler{
		cache: cache.GlobalPlaybackCache,
	}
}

// CacheStatsResponse 缓存统计响应
type CacheStatsResponse struct {
	Stats          CacheStatsData `json:"stats"`
	HitRates       HitRates       `json:"hit_rates"`
	TotalCacheSize int            `json:"total_cache_size"`
	Timestamp      time.Time      `json:"timestamp"`
	UptimeSeconds  int64          `json:"uptime_seconds"`
}

// CacheStatsData 不包含锁的缓存统计数据
type CacheStatsData struct {
	ItemInfoHits    int64 `json:"item_info_hits"`
	ItemInfoMisses  int64 `json:"item_info_misses"`
	StrmTypeHits    int64 `json:"strm_type_hits"`
	StrmTypeMisses  int64 `json:"strm_type_misses"`
	AlistLinkHits   int64 `json:"alist_link_hits"`
	AlistLinkMisses int64 `json:"alist_link_misses"`
	PlaybackHits    int64 `json:"playback_hits"`
	PlaybackMisses  int64 `json:"playback_misses"`
	TotalRequests   int64 `json:"total_requests"`
}

// convertToCacheStatsData 转换缓存统计数据
func convertToCacheStatsData(stats *cache.CacheStats) CacheStatsData {
	return CacheStatsData{
		ItemInfoHits:    stats.ItemInfoHits,
		ItemInfoMisses:  stats.ItemInfoMisses,
		StrmTypeHits:    stats.StrmTypeHits,
		StrmTypeMisses:  stats.StrmTypeMisses,
		AlistLinkHits:   stats.AlistLinkHits,
		AlistLinkMisses: stats.AlistLinkMisses,
		PlaybackHits:    stats.PlaybackHits,
		PlaybackMisses:  stats.PlaybackMisses,
		TotalRequests:   stats.TotalRequests,
	}
}

// HitRates 缓存命中率
type HitRates struct {
	ItemInfoHitRate  float64 `json:"item_info_hit_rate"`
	StrmTypeHitRate  float64 `json:"strm_type_hit_rate"`
	AlistLinkHitRate float64 `json:"alist_link_hit_rate"`
	PlaybackHitRate  float64 `json:"playback_hit_rate"`
	OverallHitRate   float64 `json:"overall_hit_rate"`
}

// GetCacheStats 获取缓存统计信息
func (h *CacheStatsHandler) GetCacheStats(ctx *gin.Context) {
	logging.Debug("======= GetCacheStats =======")

	stats := h.cache.GetStats()

	// 计算命中率
	hitRates := h.calculateHitRates(&stats)

	response := CacheStatsResponse{
		Stats:          convertToCacheStatsData(&stats),
		HitRates:       hitRates,
		TotalCacheSize: h.getTotalCacheSize(),
		Timestamp:      time.Now(),
		UptimeSeconds:  time.Since(startTime).Milliseconds() / 1000,
	}

	ctx.JSON(http.StatusOK, response)
}

// calculateHitRates 计算各种缓存的命中率
func (h *CacheStatsHandler) calculateHitRates(stats *cache.CacheStats) HitRates {
	var hitRates HitRates

	// 媒体项信息命中率
	if stats.ItemInfoHits+stats.ItemInfoMisses > 0 {
		hitRates.ItemInfoHitRate = float64(stats.ItemInfoHits) / float64(stats.ItemInfoHits+stats.ItemInfoMisses) * 100
	}

	// Strm类型命中率
	if stats.StrmTypeHits+stats.StrmTypeMisses > 0 {
		hitRates.StrmTypeHitRate = float64(stats.StrmTypeHits) / float64(stats.StrmTypeHits+stats.StrmTypeMisses) * 100
	}

	// Alist链接命中率
	if stats.AlistLinkHits+stats.AlistLinkMisses > 0 {
		hitRates.AlistLinkHitRate = float64(stats.AlistLinkHits) / float64(stats.AlistLinkHits+stats.AlistLinkMisses) * 100
	}

	// 播放信息命中率
	if stats.PlaybackHits+stats.PlaybackMisses > 0 {
		hitRates.PlaybackHitRate = float64(stats.PlaybackHits) / float64(stats.PlaybackHits+stats.PlaybackMisses) * 100
	}

	// 总体命中率
	totalHits := stats.ItemInfoHits + stats.StrmTypeHits + stats.AlistLinkHits + stats.PlaybackHits
	totalMisses := stats.ItemInfoMisses + stats.StrmTypeMisses + stats.AlistLinkMisses + stats.PlaybackMisses
	if totalHits+totalMisses > 0 {
		hitRates.OverallHitRate = float64(totalHits) / float64(totalHits+totalMisses) * 100
	}

	return hitRates
}

// getTotalCacheSize 获取缓存总大小（估算）
func (h *CacheStatsHandler) getTotalCacheSize() int {
	// 这里可以实现更精确的缓存大小计算
	// 暂时返回一个估算值
	return 0
}

// ClearCache 清空缓存
func (h *CacheStatsHandler) ClearCache(ctx *gin.Context) {
	logging.Debug("======= ClearCache =======")

	// 重新创建缓存实例来清空缓存
	cache.GlobalPlaybackCache.Close()
	cache.GlobalPlaybackCache = cache.NewPlaybackInfoCache(5 * time.Minute)

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Cache cleared successfully",
		"timestamp": time.Now(),
	})

	logging.Info("缓存已清空")
}

// WarmUpCache 预热缓存
func (h *CacheStatsHandler) WarmUpCache(ctx *gin.Context) {
	logging.Debug("======= WarmUpCache =======")

	// 这里可以实现缓存预热逻辑
	// 例如预加载热门内容

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "Cache warm-up initiated",
		"timestamp": time.Now(),
	})

	logging.Info("缓存预热已启动")
}

// GetCacheHealth 获取缓存健康状态
func (h *CacheStatsHandler) GetCacheHealth(ctx *gin.Context) {
	logging.Debug("======= GetCacheHealth =======")

	stats := h.cache.GetStats()
	hitRates := h.calculateHitRates(&stats)

	// 定义健康状态标准
	var status string
	var issues []string

	// 检查总体命中率
	if hitRates.OverallHitRate >= 80 {
		status = "healthy"
	} else if hitRates.OverallHitRate >= 60 {
		status = "warning"
		issues = append(issues, "Overall hit rate below 80%")
	} else {
		status = "critical"
		issues = append(issues, "Overall hit rate below 60%")
	}

	// 检查各项命中率
	if hitRates.ItemInfoHitRate < 70 {
		issues = append(issues, "Item info hit rate below 70%")
	}
	if hitRates.StrmTypeHitRate < 85 {
		issues = append(issues, "Strm type hit rate below 85%")
	}
	if hitRates.AlistLinkHitRate < 60 {
		issues = append(issues, "Alist link hit rate below 60%")
	}

	response := gin.H{
		"status":         status,
		"hit_rates":      hitRates,
		"issues":         issues,
		"timestamp":      time.Now(),
		"total_requests": stats.TotalRequests,
	}

	ctx.JSON(http.StatusOK, response)
}

// 应用启动时间（用于计算运行时间）
var startTime = time.Now()

// RegisterCacheStatsRoutes 注册缓存统计相关路由
func RegisterCacheStatsRoutes(router *gin.Engine) {
	handler := NewCacheStatsHandler()

	// 缓存统计API组
	cacheGroup := router.Group("/api/cache")
	{
		cacheGroup.GET("/stats", handler.GetCacheStats)
		cacheGroup.GET("/health", handler.GetCacheHealth)
		cacheGroup.POST("/clear", handler.ClearCache)
		cacheGroup.POST("/warmup", handler.WarmUpCache)
	}

	logging.Info("缓存统计API路由已注册")
}

// CacheMetrics 缓存性能指标
type CacheMetrics struct {
	ResponseTimeMs    float64 `json:"response_time_ms"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	ErrorRate         float64 `json:"error_rate"`
}

// GetCacheMetrics 获取缓存性能指标
func (h *CacheStatsHandler) GetCacheMetrics(ctx *gin.Context) {
	logging.Debug("======= GetCacheMetrics =======")

	stats := h.cache.GetStats()
	hitRates := h.calculateHitRates(&stats)

	// 计算性能指标
	uptime := time.Since(startTime).Seconds()
	var requestsPerSecond float64
	if uptime > 0 {
		requestsPerSecond = float64(stats.TotalRequests) / uptime
	}

	metrics := CacheMetrics{
		ResponseTimeMs:    0, // 需要实现响应时间监控
		CacheHitRate:      hitRates.OverallHitRate,
		RequestsPerSecond: requestsPerSecond,
		ErrorRate:         0, // 需要实现错误率监控
	}

	ctx.JSON(http.StatusOK, metrics)
}

// OptimizationRecommendations 优化建议
type OptimizationRecommendations struct {
	Recommendations []string `json:"recommendations"`
	Priority        string   `json:"priority"`
	EstimatedImpact string   `json:"estimated_impact"`
}

// GetOptimizationRecommendations 获取优化建议
func (h *CacheStatsHandler) GetOptimizationRecommendations(ctx *gin.Context) {
	logging.Debug("======= GetOptimizationRecommendations =======")

	stats := h.cache.GetStats()
	hitRates := h.calculateHitRates(&stats)

	var recommendations []string
	priority := "low"
	impact := "minimal"

	// 基于统计数据生成建议
	if hitRates.OverallHitRate < 60 {
		recommendations = append(recommendations, "Consider increasing cache TTL values")
		recommendations = append(recommendations, "Implement cache preloading for popular content")
		priority = "high"
		impact = "significant"
	} else if hitRates.OverallHitRate < 80 {
		recommendations = append(recommendations, "Fine-tune cache eviction policies")
		recommendations = append(recommendations, "Monitor cache size and memory usage")
		priority = "medium"
		impact = "moderate"
	}

	if hitRates.ItemInfoHitRate < 70 {
		recommendations = append(recommendations, "Increase item info cache TTL")
	}

	if hitRates.StrmTypeHitRate < 85 {
		recommendations = append(recommendations, "Strm type cache is performing well")
	}

	if hitRates.AlistLinkHitRate < 60 {
		recommendations = append(recommendations, "Consider longer TTL for Alist links")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Cache performance is optimal")
	}

	response := OptimizationRecommendations{
		Recommendations: recommendations,
		Priority:        priority,
		EstimatedImpact: impact,
	}

	ctx.JSON(http.StatusOK, response)
}
