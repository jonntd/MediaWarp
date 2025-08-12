package health

import (
	"context"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusDegraded  HealthStatus = "degraded"
)

// CheckResult 检查结果
type CheckResult struct {
	Name     string                 `json:"name"`
	Status   HealthStatus           `json:"status"`
	Message  string                 `json:"message,omitempty"`
	Duration time.Duration          `json:"duration"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// HealthChecker 健康检查器
type HealthChecker struct {
	checks []HealthCheck
	mutex  sync.RWMutex
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make([]HealthCheck, 0),
	}
}

// AddCheck 添加健康检查
func (hc *HealthChecker) AddCheck(check HealthCheck) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.checks = append(hc.checks, check)
}

// CheckAll 执行所有健康检查
func (hc *HealthChecker) CheckAll(ctx context.Context) map[string]CheckResult {
	hc.mutex.RLock()
	checks := make([]HealthCheck, len(hc.checks))
	copy(checks, hc.checks)
	hc.mutex.RUnlock()

	results := make(map[string]CheckResult)
	var wg sync.WaitGroup

	for _, check := range checks {
		wg.Add(1)
		go func(c HealthCheck) {
			defer wg.Done()
			result := c.Check(ctx)
			hc.mutex.Lock()
			results[c.Name()] = result
			hc.mutex.Unlock()
		}(check)
	}

	wg.Wait()
	return results
}

// GetOverallStatus 获取整体健康状态
func (hc *HealthChecker) GetOverallStatus(results map[string]CheckResult) HealthStatus {
	hasUnhealthy := false
	hasDegraded := false

	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// RcloneHealthCheck rclone健康检查
type RcloneHealthCheck struct{}

func (r *RcloneHealthCheck) Name() string {
	return "rclone"
}

func (r *RcloneHealthCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()

	// 检查rclone是否可用
	cmd := exec.CommandContext(ctx, "rclone", "version")
	err := cmd.Run()

	duration := time.Since(start)

	if err != nil {
		return CheckResult{
			Name:     r.Name(),
			Status:   StatusUnhealthy,
			Message:  "rclone is not available",
			Duration: duration,
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return CheckResult{
		Name:     r.Name(),
		Status:   StatusHealthy,
		Message:  "rclone is available",
		Duration: duration,
	}
}

// CacheHealthCheck 缓存健康检查
type CacheHealthCheck struct {
	cacheSize func() int
}

func NewCacheHealthCheck(cacheSize func() int) *CacheHealthCheck {
	return &CacheHealthCheck{cacheSize: cacheSize}
}

func (c *CacheHealthCheck) Name() string {
	return "cache"
}

func (c *CacheHealthCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()

	size := c.cacheSize()
	duration := time.Since(start)

	status := StatusHealthy
	message := "cache is healthy"

	// 如果缓存过大，标记为降级
	if size > 10000 {
		status = StatusDegraded
		message = "cache size is large"
	}

	return CheckResult{
		Name:     c.Name(),
		Status:   status,
		Message:  message,
		Duration: duration,
		Details: map[string]interface{}{
			"size": size,
		},
	}
}

// ProcessHealthCheck 进程健康检查
type ProcessHealthCheck struct {
	getProcessCount func() int
}

func NewProcessHealthCheck(getProcessCount func() int) *ProcessHealthCheck {
	return &ProcessHealthCheck{getProcessCount: getProcessCount}
}

func (p *ProcessHealthCheck) Name() string {
	return "processes"
}

func (p *ProcessHealthCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()

	count := p.getProcessCount()
	duration := time.Since(start)

	status := StatusHealthy
	message := "process count is normal"

	// 如果进程过多，标记为降级
	if count > 50 {
		status = StatusDegraded
		message = "high process count"
	} else if count > 100 {
		status = StatusUnhealthy
		message = "very high process count"
	}

	return CheckResult{
		Name:     p.Name(),
		Status:   status,
		Message:  message,
		Duration: duration,
		Details: map[string]interface{}{
			"count": count,
		},
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
	Summary   map[string]interface{} `json:"summary"`
}

// 全局健康检查器
var GlobalHealthChecker = NewHealthChecker()

// 初始化默认健康检查
func init() {
	GlobalHealthChecker.AddCheck(&RcloneHealthCheck{})
}

// HealthHandler 健康检查HTTP处理器
func HealthHandler(ctx *gin.Context) {
	checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := GlobalHealthChecker.CheckAll(checkCtx)
	overallStatus := GlobalHealthChecker.GetOverallStatus(results)

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    results,
		Summary: map[string]interface{}{
			"total_checks": len(results),
			"healthy":      countByStatus(results, StatusHealthy),
			"degraded":     countByStatus(results, StatusDegraded),
			"unhealthy":    countByStatus(results, StatusUnhealthy),
		},
	}

	// 根据整体状态设置HTTP状态码
	statusCode := http.StatusOK
	if overallStatus == StatusDegraded {
		statusCode = http.StatusOK // 降级状态仍返回200
	} else if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	ctx.JSON(statusCode, response)
}

// ReadinessHandler 就绪检查处理器
func ReadinessHandler(ctx *gin.Context) {
	// 简单的就绪检查，只检查关键组件
	checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rcloneCheck := &RcloneHealthCheck{}
	result := rcloneCheck.Check(checkCtx)

	if result.Status == StatusUnhealthy {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not_ready",
			"message": "rclone is not available",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"message": "service is ready",
	})
}

// LivenessHandler 存活检查处理器
func LivenessHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now(),
	})
}

// countByStatus 按状态计数
func countByStatus(results map[string]CheckResult, status HealthStatus) int {
	count := 0
	for _, result := range results {
		if result.Status == status {
			count++
		}
	}
	return count
}
