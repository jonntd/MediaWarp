package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// RequestDeduplicator 请求去重器
type RequestDeduplicator struct {
	// 正在进行的请求
	pendingRequests map[string]*PendingRequest
	mutex           sync.RWMutex

	// 去重统计
	stats DeduplicationStats

	// 配置
	config DeduplicationConfig
}

// PendingRequest 待处理请求
type PendingRequest struct {
	// 等待结果的通道列表
	waiters []chan RequestResult

	// 请求开始时间
	startTime time.Time

	// 请求参数（预留字段）
	// params RequestParams

	// 互斥锁
	mutex sync.Mutex
}

// RequestResult 请求结果
type RequestResult struct {
	Data  interface{}
	Error error
}

// RequestParams 请求参数
type RequestParams struct {
	MediaSourceID string
	Fields        string
	Limit         int
}

// DeduplicationStats 去重统计
type DeduplicationStats struct {
	TotalRequests     int64         `json:"total_requests"`
	DeduplicatedCount int64         `json:"deduplicated_count"`
	SavedTime         time.Duration `json:"saved_time"`
	AverageWaitTime   time.Duration `json:"average_wait_time"`
	mutex             sync.RWMutex
}

// DeduplicationConfig 去重配置
type DeduplicationConfig struct {
	Enabled            bool          `json:"enabled"`
	MaxWaitTime        time.Duration `json:"max_wait_time"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	MaxPendingRequests int           `json:"max_pending_requests"`
}

// NewRequestDeduplicator 创建请求去重器
func NewRequestDeduplicator() *RequestDeduplicator {
	rd := &RequestDeduplicator{
		pendingRequests: make(map[string]*PendingRequest),
		config: DeduplicationConfig{
			Enabled:            true,
			MaxWaitTime:        5 * time.Second,
			CleanupInterval:    1 * time.Minute,
			MaxPendingRequests: 1000,
		},
		stats: DeduplicationStats{},
	}

	// 启动清理协程
	go rd.startCleanup()

	return rd
}

// Do 执行去重请求
func (rd *RequestDeduplicator) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	if !rd.config.Enabled {
		// 去重禁用，直接执行
		return fn()
	}

	rd.stats.incrementTotalRequests()

	// 生成请求键
	requestKey := rd.generateRequestKey(key)

	// 检查是否有正在进行的相同请求
	rd.mutex.RLock()
	pending, exists := rd.pendingRequests[requestKey]
	rd.mutex.RUnlock()

	if exists {
		// 有相同请求正在进行，等待结果
		return rd.waitForResult(pending, requestKey)
	}

	// 没有相同请求，创建新的请求
	return rd.executeNewRequest(requestKey, fn)
}

// waitForResult 等待请求结果
func (rd *RequestDeduplicator) waitForResult(pending *PendingRequest, requestKey string) (interface{}, error) {
	rd.stats.incrementDeduplicatedCount()

	// 创建等待通道
	resultChan := make(chan RequestResult, 1)

	// 添加到等待列表
	pending.mutex.Lock()
	pending.waiters = append(pending.waiters, resultChan)
	pending.mutex.Unlock()

	// 等待结果或超时
	select {
	case result := <-resultChan:
		return result.Data, result.Error
	case <-time.After(rd.config.MaxWaitTime):
		// 超时，移除等待者并执行新请求
		rd.removeWaiter(pending, resultChan)
		return nil, fmt.Errorf("request deduplication timeout")
	}
}

// executeNewRequest 执行新请求
func (rd *RequestDeduplicator) executeNewRequest(requestKey string, fn func() (interface{}, error)) (interface{}, error) {
	// 创建新的待处理请求
	pending := &PendingRequest{
		waiters:   make([]chan RequestResult, 0),
		startTime: time.Now(),
	}

	// 注册请求
	rd.mutex.Lock()
	if len(rd.pendingRequests) >= rd.config.MaxPendingRequests {
		rd.mutex.Unlock()
		// 达到最大并发数，直接执行
		return fn()
	}
	rd.pendingRequests[requestKey] = pending
	rd.mutex.Unlock()

	// 执行请求
	data, err := fn()

	// 通知所有等待者
	rd.notifyWaiters(pending, RequestResult{Data: data, Error: err})

	// 清理请求
	rd.mutex.Lock()
	delete(rd.pendingRequests, requestKey)
	rd.mutex.Unlock()

	// 更新统计
	duration := time.Since(pending.startTime)
	rd.stats.updateSavedTime(duration, len(pending.waiters))

	return data, err
}

// notifyWaiters 通知所有等待者
func (rd *RequestDeduplicator) notifyWaiters(pending *PendingRequest, result RequestResult) {
	pending.mutex.Lock()
	defer pending.mutex.Unlock()

	for _, waiter := range pending.waiters {
		select {
		case waiter <- result:
		default:
			// 通道已满或已关闭，跳过
		}
	}
}

// removeWaiter 移除等待者
func (rd *RequestDeduplicator) removeWaiter(pending *PendingRequest, waiter chan RequestResult) {
	pending.mutex.Lock()
	defer pending.mutex.Unlock()

	for i, w := range pending.waiters {
		if w == waiter {
			pending.waiters = append(pending.waiters[:i], pending.waiters[i+1:]...)
			break
		}
	}
}

// generateRequestKey 生成请求键
func (rd *RequestDeduplicator) generateRequestKey(key string) string {
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// startCleanup 启动清理协程
func (rd *RequestDeduplicator) startCleanup() {
	ticker := time.NewTicker(rd.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rd.cleanupExpiredRequests()
	}
}

// cleanupExpiredRequests 清理过期请求
func (rd *RequestDeduplicator) cleanupExpiredRequests() {
	rd.mutex.Lock()
	defer rd.mutex.Unlock()

	now := time.Now()
	for key, pending := range rd.pendingRequests {
		if now.Sub(pending.startTime) > rd.config.MaxWaitTime*2 {
			// 请求超时，清理
			delete(rd.pendingRequests, key)
		}
	}
}

// GetStats 获取去重统计
func (rd *RequestDeduplicator) GetStats() DeduplicationStats {
	rd.stats.mutex.RLock()
	defer rd.stats.mutex.RUnlock()

	return DeduplicationStats{
		TotalRequests:     rd.stats.TotalRequests,
		DeduplicatedCount: rd.stats.DeduplicatedCount,
		SavedTime:         rd.stats.SavedTime,
		AverageWaitTime:   rd.stats.AverageWaitTime,
	}
}

// 统计方法
func (ds *DeduplicationStats) incrementTotalRequests() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.TotalRequests++
}

func (ds *DeduplicationStats) incrementDeduplicatedCount() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.DeduplicatedCount++
}

func (ds *DeduplicationStats) updateSavedTime(duration time.Duration, waiterCount int) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// 计算节省的时间（等待者数量 * 请求时间）
	savedTime := duration * time.Duration(waiterCount)
	ds.SavedTime += savedTime

	// 更新平均等待时间
	if ds.DeduplicatedCount > 0 {
		ds.AverageWaitTime = ds.SavedTime / time.Duration(ds.DeduplicatedCount)
	}
}

// BatchRequestDeduplicator 批量请求去重器
type BatchRequestDeduplicator struct {
	*RequestDeduplicator

	// 批量配置
	batchConfig BatchConfig

	// 批量队列
	batchQueue map[string]*BatchRequest
	batchMutex sync.RWMutex
}

// BatchConfig 批量配置
type BatchConfig struct {
	Enabled       bool          `json:"enabled"`
	BatchSize     int           `json:"batch_size"`
	BatchTimeout  time.Duration `json:"batch_timeout"`
	MaxBatchCount int           `json:"max_batch_count"`
}

// BatchRequest 批量请求
type BatchRequest struct {
	requests []BatchRequestItem
	timer    *time.Timer
	mutex    sync.Mutex
}

// BatchRequestItem 批量请求项
type BatchRequestItem struct {
	Key        string
	ResultChan chan RequestResult
}

// NewBatchRequestDeduplicator 创建批量请求去重器
func NewBatchRequestDeduplicator() *BatchRequestDeduplicator {
	return &BatchRequestDeduplicator{
		RequestDeduplicator: NewRequestDeduplicator(),
		batchConfig: BatchConfig{
			Enabled:       true,
			BatchSize:     10,
			BatchTimeout:  100 * time.Millisecond,
			MaxBatchCount: 100,
		},
		batchQueue: make(map[string]*BatchRequest),
	}
}

// DoBatch 执行批量去重请求
func (brd *BatchRequestDeduplicator) DoBatch(batchKey string, itemKey string, fn func([]string) (map[string]interface{}, error)) (interface{}, error) {
	if !brd.batchConfig.Enabled {
		// 批量处理禁用，使用普通去重
		return brd.Do(itemKey, func() (interface{}, error) {
			results, err := fn([]string{itemKey})
			if err != nil {
				return nil, err
			}
			return results[itemKey], nil
		})
	}

	// 添加到批量队列
	resultChan := make(chan RequestResult, 1)

	brd.batchMutex.Lock()
	batch, exists := brd.batchQueue[batchKey]
	if !exists {
		batch = &BatchRequest{
			requests: make([]BatchRequestItem, 0),
		}
		brd.batchQueue[batchKey] = batch

		// 设置批量超时
		batch.timer = time.AfterFunc(brd.batchConfig.BatchTimeout, func() {
			brd.executeBatch(batchKey, fn)
		})
	}
	brd.batchMutex.Unlock()

	// 添加请求到批量
	batch.mutex.Lock()
	batch.requests = append(batch.requests, BatchRequestItem{
		Key:        itemKey,
		ResultChan: resultChan,
	})

	// 检查是否达到批量大小
	if len(batch.requests) >= brd.batchConfig.BatchSize {
		batch.timer.Stop()
		batch.mutex.Unlock()
		go brd.executeBatch(batchKey, fn)
	} else {
		batch.mutex.Unlock()
	}

	// 等待结果
	result := <-resultChan
	return result.Data, result.Error
}

// executeBatch 执行批量请求
func (brd *BatchRequestDeduplicator) executeBatch(batchKey string, fn func([]string) (map[string]interface{}, error)) {
	brd.batchMutex.Lock()
	batch, exists := brd.batchQueue[batchKey]
	if !exists {
		brd.batchMutex.Unlock()
		return
	}
	delete(brd.batchQueue, batchKey)
	brd.batchMutex.Unlock()

	batch.mutex.Lock()
	requests := make([]BatchRequestItem, len(batch.requests))
	copy(requests, batch.requests)
	batch.mutex.Unlock()

	if len(requests) == 0 {
		return
	}

	// 提取所有键
	keys := make([]string, len(requests))
	for i, req := range requests {
		keys[i] = req.Key
	}

	// 执行批量请求
	results, err := fn(keys)

	// 分发结果
	for _, req := range requests {
		var result RequestResult
		if err != nil {
			result = RequestResult{Error: err}
		} else if data, exists := results[req.Key]; exists {
			result = RequestResult{Data: data}
		} else {
			result = RequestResult{Error: fmt.Errorf("no result for key: %s", req.Key)}
		}

		select {
		case req.ResultChan <- result:
		default:
			// 通道已满，跳过
		}
	}
}

// 全局请求去重器实例
var GlobalRequestDeduplicator = NewRequestDeduplicator()
var GlobalBatchRequestDeduplicator = NewBatchRequestDeduplicator()
