package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

// Metric 指标接口
type Metric interface {
	Name() string
	Type() MetricType
	Value() interface{}
	Labels() map[string]string
}

// CounterMetric 计数器指标
type CounterMetric struct {
	name   string
	value  int64
	labels map[string]string
}

func (c *CounterMetric) Name() string                 { return c.name }
func (c *CounterMetric) Type() MetricType             { return Counter }
func (c *CounterMetric) Value() interface{}           { return atomic.LoadInt64(&c.value) }
func (c *CounterMetric) Labels() map[string]string    { return c.labels }
func (c *CounterMetric) Inc()                         { atomic.AddInt64(&c.value, 1) }
func (c *CounterMetric) Add(delta int64)              { atomic.AddInt64(&c.value, delta) }

// GaugeMetric 仪表盘指标
type GaugeMetric struct {
	name   string
	value  int64
	labels map[string]string
}

func (g *GaugeMetric) Name() string                 { return g.name }
func (g *GaugeMetric) Type() MetricType             { return Gauge }
func (g *GaugeMetric) Value() interface{}           { return atomic.LoadInt64(&g.value) }
func (g *GaugeMetric) Labels() map[string]string    { return g.labels }
func (g *GaugeMetric) Set(value int64)              { atomic.StoreInt64(&g.value, value) }
func (g *GaugeMetric) Inc()                         { atomic.AddInt64(&g.value, 1) }
func (g *GaugeMetric) Dec()                         { atomic.AddInt64(&g.value, -1) }

// HistogramMetric 直方图指标
type HistogramMetric struct {
	name    string
	buckets []float64
	counts  []int64
	sum     int64
	count   int64
	labels  map[string]string
	mutex   sync.RWMutex
}

func (h *HistogramMetric) Name() string              { return h.name }
func (h *HistogramMetric) Type() MetricType          { return Histogram }
func (h *HistogramMetric) Labels() map[string]string { return h.labels }

func (h *HistogramMetric) Value() interface{} {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	return map[string]interface{}{
		"buckets": h.buckets,
		"counts":  h.counts,
		"sum":     atomic.LoadInt64(&h.sum),
		"count":   atomic.LoadInt64(&h.count),
	}
}

func (h *HistogramMetric) Observe(value float64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, int64(value*1000)) // 存储毫秒
	
	for i, bucket := range h.buckets {
		if value <= bucket {
			atomic.AddInt64(&h.counts[i], 1)
		}
	}
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	metrics map[string]Metric
	mutex   sync.RWMutex
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]Metric),
	}
}

// RegisterCounter 注册计数器
func (mc *MetricsCollector) RegisterCounter(name string, labels map[string]string) *CounterMetric {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	counter := &CounterMetric{
		name:   name,
		labels: labels,
	}
	mc.metrics[name] = counter
	return counter
}

// RegisterGauge 注册仪表盘
func (mc *MetricsCollector) RegisterGauge(name string, labels map[string]string) *GaugeMetric {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	gauge := &GaugeMetric{
		name:   name,
		labels: labels,
	}
	mc.metrics[name] = gauge
	return gauge
}

// RegisterHistogram 注册直方图
func (mc *MetricsCollector) RegisterHistogram(name string, buckets []float64, labels map[string]string) *HistogramMetric {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	histogram := &HistogramMetric{
		name:    name,
		buckets: buckets,
		counts:  make([]int64, len(buckets)),
		labels:  labels,
	}
	mc.metrics[name] = histogram
	return histogram
}

// GetMetric 获取指标
func (mc *MetricsCollector) GetMetric(name string) (Metric, bool) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	metric, exists := mc.metrics[name]
	return metric, exists
}

// GetAllMetrics 获取所有指标
func (mc *MetricsCollector) GetAllMetrics() map[string]Metric {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	result := make(map[string]Metric)
	for name, metric := range mc.metrics {
		result[name] = metric
	}
	return result
}

// 全局指标收集器
var GlobalCollector = NewMetricsCollector()

// 预定义指标
var (
	// HTTP请求指标
	HTTPRequestsTotal = GlobalCollector.RegisterCounter("http_requests_total", map[string]string{
		"method": "",
		"path":   "",
		"status": "",
	})
	
	HTTPRequestDuration = GlobalCollector.RegisterHistogram("http_request_duration_seconds", 
		[]float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0}, 
		map[string]string{"method": "", "path": ""})
	
	// 缓存指标
	CacheHits   = GlobalCollector.RegisterCounter("cache_hits_total", nil)
	CacheMisses = GlobalCollector.RegisterCounter("cache_misses_total", nil)
	CacheSize   = GlobalCollector.RegisterGauge("cache_size", nil)
	
	// 进程指标
	ActiveProcesses = GlobalCollector.RegisterGauge("active_processes", nil)
	ProcessErrors   = GlobalCollector.RegisterCounter("process_errors_total", nil)
	
	// 同步指标
	SyncOperations = GlobalCollector.RegisterCounter("sync_operations_total", map[string]string{
		"type":   "",
		"status": "",
	})
	
	SyncDuration = GlobalCollector.RegisterHistogram("sync_duration_seconds",
		[]float64{1.0, 5.0, 10.0, 30.0, 60.0, 300.0},
		map[string]string{"type": ""})
)

// RecordHTTPRequest 记录HTTP请求
func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	// 更新计数器
	counter := GlobalCollector.RegisterCounter("http_requests_total", map[string]string{
		"method": method,
		"path":   path,
		"status": string(rune(status)),
	})
	counter.Inc()
	
	// 更新直方图
	histogram := GlobalCollector.RegisterHistogram("http_request_duration_seconds",
		[]float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
		map[string]string{"method": method, "path": path})
	histogram.Observe(duration.Seconds())
}

// RecordCacheHit 记录缓存命中
func RecordCacheHit() {
	CacheHits.Inc()
}

// RecordCacheMiss 记录缓存未命中
func RecordCacheMiss() {
	CacheMisses.Inc()
}

// UpdateCacheSize 更新缓存大小
func UpdateCacheSize(size int64) {
	CacheSize.Set(size)
}

// RecordProcessStart 记录进程启动
func RecordProcessStart() {
	ActiveProcesses.Inc()
}

// RecordProcessEnd 记录进程结束
func RecordProcessEnd() {
	ActiveProcesses.Dec()
}

// RecordProcessError 记录进程错误
func RecordProcessError() {
	ProcessErrors.Inc()
}

// RecordSyncOperation 记录同步操作
func RecordSyncOperation(syncType, status string, duration time.Duration) {
	// 更新计数器
	counter := GlobalCollector.RegisterCounter("sync_operations_total", map[string]string{
		"type":   syncType,
		"status": status,
	})
	counter.Inc()
	
	// 更新直方图
	histogram := GlobalCollector.RegisterHistogram("sync_duration_seconds",
		[]float64{1.0, 5.0, 10.0, 30.0, 60.0, 300.0},
		map[string]string{"type": syncType})
	histogram.Observe(duration.Seconds())
}
