package config

import (
	"runtime"
	"time"
)

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	// 缓存配置
	Cache CacheConfig `yaml:"cache" json:"cache"`
	
	// HTTP配置
	HTTP HTTPConfig `yaml:"http" json:"http"`
	
	// 进程配置
	Process ProcessConfig `yaml:"process" json:"process"`
	
	// 监控配置
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 默认缓存时间
	DefaultTTL time.Duration `yaml:"default_ttl" json:"default_ttl"`
	
	// 清理间隔
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	
	// 最大缓存项数
	MaxItems int `yaml:"max_items" json:"max_items"`
	
	// 最大内存使用（MB）
	MaxMemoryMB int `yaml:"max_memory_mb" json:"max_memory_mb"`
}

// HTTPConfig HTTP配置
type HTTPConfig struct {
	// 连接超时
	ConnectTimeout time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	
	// 读取超时
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	
	// 写入超时
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	
	// 空闲超时
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	
	// 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`
	
	// 每个主机最大空闲连接数
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`
	
	// 最大连接数
	MaxConnsPerHost int `yaml:"max_conns_per_host" json:"max_conns_per_host"`
	
	// 启用压缩
	EnableCompression bool `yaml:"enable_compression" json:"enable_compression"`
}

// ProcessConfig 进程配置
type ProcessConfig struct {
	// 最大并发进程数
	MaxConcurrentProcesses int `yaml:"max_concurrent_processes" json:"max_concurrent_processes"`
	
	// 进程超时
	ProcessTimeout time.Duration `yaml:"process_timeout" json:"process_timeout"`
	
	// 进程清理间隔
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	
	// 启用进程池
	EnableProcessPool bool `yaml:"enable_process_pool" json:"enable_process_pool"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	// 启用指标收集
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics"`
	
	// 指标收集间隔
	MetricsInterval time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	
	// 启用健康检查
	EnableHealthCheck bool `yaml:"enable_health_check" json:"enable_health_check"`
	
	// 健康检查间隔
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	
	// 启用详细日志
	EnableVerboseLogging bool `yaml:"enable_verbose_logging" json:"enable_verbose_logging"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	// 启用速率限制
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// 每分钟请求数
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requests_per_minute"`
	
	// 突发请求数
	BurstSize int `yaml:"burst_size" json:"burst_size"`
	
	// 清理间隔
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 启用安全头
	EnableSecurityHeaders bool `yaml:"enable_security_headers" json:"enable_security_headers"`
	
	// 启用CORS
	EnableCORS bool `yaml:"enable_cors" json:"enable_cors"`
	
	// 允许的源
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
	
	// 启用请求验证
	EnableRequestValidation bool `yaml:"enable_request_validation" json:"enable_request_validation"`
	
	// 最大请求体大小（MB）
	MaxRequestBodyMB int `yaml:"max_request_body_mb" json:"max_request_body_mb"`
}

// DefaultPerformanceConfig 默认性能配置
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		Cache: CacheConfig{
			DefaultTTL:      15 * time.Minute,
			CleanupInterval: 5 * time.Minute,
			MaxItems:        10000,
			MaxMemoryMB:     100,
		},
		HTTP: HTTPConfig{
			ConnectTimeout:          10 * time.Second,
			ReadTimeout:             30 * time.Second,
			WriteTimeout:            30 * time.Second,
			IdleTimeout:             90 * time.Second,
			MaxIdleConns:            100,
			MaxIdleConnsPerHost:     10,
			MaxConnsPerHost:         50,
			EnableCompression:       true,
		},
		Process: ProcessConfig{
			MaxConcurrentProcesses: runtime.NumCPU() * 2,
			ProcessTimeout:         5 * time.Minute,
			CleanupInterval:        1 * time.Minute,
			EnableProcessPool:      true,
		},
		Monitoring: MonitoringConfig{
			EnableMetrics:           true,
			MetricsInterval:         30 * time.Second,
			EnableHealthCheck:       true,
			HealthCheckInterval:     1 * time.Minute,
			EnableVerboseLogging:    false,
		},
	}
}

// DefaultRateLimitConfig 默认速率限制配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 100,
		BurstSize:         20,
		CleanupInterval:   1 * time.Minute,
	}
}

// DefaultSecurityConfig 默认安全配置
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		EnableSecurityHeaders:   true,
		EnableCORS:             true,
		AllowedOrigins:         []string{"*"},
		EnableRequestValidation: true,
		MaxRequestBodyMB:       10,
	}
}

// OptimizeForEnvironment 根据环境优化配置
func (pc *PerformanceConfig) OptimizeForEnvironment(env string) {
	switch env {
	case "development":
		pc.optimizeForDevelopment()
	case "testing":
		pc.optimizeForTesting()
	case "production":
		pc.optimizeForProduction()
	default:
		// 使用默认配置
	}
}

// optimizeForDevelopment 开发环境优化
func (pc *PerformanceConfig) optimizeForDevelopment() {
	// 缓存时间较短，便于调试
	pc.Cache.DefaultTTL = 5 * time.Minute
	pc.Cache.CleanupInterval = 1 * time.Minute
	
	// 启用详细日志
	pc.Monitoring.EnableVerboseLogging = true
	pc.Monitoring.MetricsInterval = 10 * time.Second
	
	// 较短的超时时间
	pc.HTTP.ConnectTimeout = 5 * time.Second
	pc.HTTP.ReadTimeout = 15 * time.Second
	pc.Process.ProcessTimeout = 1 * time.Minute
}

// optimizeForTesting 测试环境优化
func (pc *PerformanceConfig) optimizeForTesting() {
	// 快速缓存清理
	pc.Cache.DefaultTTL = 1 * time.Minute
	pc.Cache.CleanupInterval = 30 * time.Second
	
	// 频繁的健康检查
	pc.Monitoring.HealthCheckInterval = 30 * time.Second
	pc.Monitoring.MetricsInterval = 15 * time.Second
	
	// 较短的超时时间
	pc.HTTP.ConnectTimeout = 3 * time.Second
	pc.Process.ProcessTimeout = 30 * time.Second
}

// optimizeForProduction 生产环境优化
func (pc *PerformanceConfig) optimizeForProduction() {
	// 较长的缓存时间
	pc.Cache.DefaultTTL = 30 * time.Minute
	pc.Cache.CleanupInterval = 10 * time.Minute
	pc.Cache.MaxItems = 50000
	pc.Cache.MaxMemoryMB = 500
	
	// 优化HTTP连接
	pc.HTTP.MaxIdleConns = 200
	pc.HTTP.MaxIdleConnsPerHost = 20
	pc.HTTP.MaxConnsPerHost = 100
	
	// 增加进程并发数
	pc.Process.MaxConcurrentProcesses = runtime.NumCPU() * 4
	
	// 关闭详细日志
	pc.Monitoring.EnableVerboseLogging = false
	pc.Monitoring.MetricsInterval = 1 * time.Minute
	pc.Monitoring.HealthCheckInterval = 2 * time.Minute
}

// Validate 验证性能配置
func (pc *PerformanceConfig) Validate() error {
	if pc.Cache.DefaultTTL <= 0 {
		pc.Cache.DefaultTTL = 15 * time.Minute
	}
	
	if pc.Cache.CleanupInterval <= 0 {
		pc.Cache.CleanupInterval = 5 * time.Minute
	}
	
	if pc.HTTP.ConnectTimeout <= 0 {
		pc.HTTP.ConnectTimeout = 10 * time.Second
	}
	
	if pc.Process.MaxConcurrentProcesses <= 0 {
		pc.Process.MaxConcurrentProcesses = runtime.NumCPU()
	}
	
	if pc.Process.ProcessTimeout <= 0 {
		pc.Process.ProcessTimeout = 5 * time.Minute
	}
	
	return nil
}

// GetMemoryLimit 获取内存限制（字节）
func (pc *PerformanceConfig) GetMemoryLimit() int64 {
	return int64(pc.Cache.MaxMemoryMB) * 1024 * 1024
}

// GetMaxConnections 获取最大连接数
func (pc *PerformanceConfig) GetMaxConnections() int {
	return pc.HTTP.MaxConnsPerHost
}

// IsProductionOptimized 检查是否为生产环境优化
func (pc *PerformanceConfig) IsProductionOptimized() bool {
	return pc.Cache.DefaultTTL >= 30*time.Minute &&
		pc.HTTP.MaxIdleConns >= 100 &&
		!pc.Monitoring.EnableVerboseLogging
}
