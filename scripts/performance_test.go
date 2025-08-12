package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// PerformanceTestConfig 性能测试配置
type PerformanceTestConfig struct {
	BaseURL         string         `json:"base_url"`
	ConcurrentUsers int            `json:"concurrent_users"`
	TestDuration    time.Duration  `json:"test_duration"`
	RequestInterval time.Duration  `json:"request_interval"`
	TestEndpoints   []TestEndpoint `json:"test_endpoints"`
}

// TestEndpoint 测试端点
type TestEndpoint struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Weight  int               `json:"weight"` // 权重，用于控制请求频率
}

// TestResult 测试结果
type TestResult struct {
	EndpointName      string        `json:"endpoint_name"`
	TotalRequests     int           `json:"total_requests"`
	SuccessRequests   int           `json:"success_requests"`
	FailedRequests    int           `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	P50Latency        time.Duration `json:"p50_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	RequestsPerSecond float64       `json:"requests_per_second"`
	ErrorRate         float64       `json:"error_rate"`
}

// PerformanceTester 性能测试器
type PerformanceTester struct {
	config  PerformanceTestConfig
	client  *http.Client
	results map[string]*TestResult
	mutex   sync.RWMutex
}

// NewPerformanceTester 创建性能测试器
func NewPerformanceTester(config PerformanceTestConfig) *PerformanceTester {
	return &PerformanceTester{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		results: make(map[string]*TestResult),
	}
}

// RunTest 运行性能测试
func (pt *PerformanceTester) RunTest() {
	fmt.Printf("开始性能测试...\n")
	fmt.Printf("并发用户数: %d\n", pt.config.ConcurrentUsers)
	fmt.Printf("测试时长: %v\n", pt.config.TestDuration)
	fmt.Printf("基础URL: %s\n", pt.config.BaseURL)

	// 初始化结果
	for _, endpoint := range pt.config.TestEndpoints {
		pt.results[endpoint.Name] = &TestResult{
			EndpointName: endpoint.Name,
			MinLatency:   time.Hour, // 初始化为很大的值
		}
	}

	var wg sync.WaitGroup
	stopChan := make(chan bool)

	// 启动并发测试
	for i := 0; i < pt.config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			pt.runUserTest(userID, stopChan)
		}(i)
	}

	// 等待测试时间结束
	time.Sleep(pt.config.TestDuration)
	close(stopChan)

	// 等待所有协程结束
	wg.Wait()

	// 输出结果
	pt.printResults()
}

// runUserTest 运行单个用户的测试
func (pt *PerformanceTester) runUserTest(userID int, stopChan chan bool) {
	ticker := time.NewTicker(pt.config.RequestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			// 随机选择一个端点进行测试
			endpoint := pt.selectEndpoint()
			pt.makeRequest(endpoint)
		}
	}
}

// selectEndpoint 根据权重选择端点
func (pt *PerformanceTester) selectEndpoint() TestEndpoint {
	// 简单实现：随机选择
	// 实际应该根据权重进行选择
	if len(pt.config.TestEndpoints) == 0 {
		return TestEndpoint{}
	}
	return pt.config.TestEndpoints[time.Now().Nanosecond()%len(pt.config.TestEndpoints)]
}

// makeRequest 发起HTTP请求
func (pt *PerformanceTester) makeRequest(endpoint TestEndpoint) {
	startTime := time.Now()

	// 构建请求
	var body io.Reader
	if endpoint.Body != "" {
		body = bytes.NewBufferString(endpoint.Body)
	}

	req, err := http.NewRequest(endpoint.Method, pt.config.BaseURL+endpoint.Path, body)
	if err != nil {
		pt.recordFailure(endpoint.Name, time.Since(startTime))
		return
	}

	// 设置请求头
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// 发起请求
	resp, err := pt.client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		pt.recordFailure(endpoint.Name, latency)
		return
	}
	defer resp.Body.Close()

	// 记录结果
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		pt.recordSuccess(endpoint.Name, latency)
	} else {
		pt.recordFailure(endpoint.Name, latency)
	}
}

// recordSuccess 记录成功请求
func (pt *PerformanceTester) recordSuccess(endpointName string, latency time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	result := pt.results[endpointName]
	result.TotalRequests++
	result.SuccessRequests++

	// 更新延迟统计
	if latency < result.MinLatency {
		result.MinLatency = latency
	}
	if latency > result.MaxLatency {
		result.MaxLatency = latency
	}

	// 简单的平均延迟计算
	result.AverageLatency = (result.AverageLatency*time.Duration(result.SuccessRequests-1) + latency) / time.Duration(result.SuccessRequests)
}

// recordFailure 记录失败请求
func (pt *PerformanceTester) recordFailure(endpointName string, latency time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	result := pt.results[endpointName]
	result.TotalRequests++
	result.FailedRequests++
}

// printResults 打印测试结果
func (pt *PerformanceTester) printResults() {
	fmt.Printf("\n======= 性能测试结果 =======\n")

	for _, result := range pt.results {
		if result.TotalRequests == 0 {
			continue
		}

		// 计算统计数据
		result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests) * 100
		result.RequestsPerSecond = float64(result.TotalRequests) / pt.config.TestDuration.Seconds()

		fmt.Printf("\n端点: %s\n", result.EndpointName)
		fmt.Printf("  总请求数: %d\n", result.TotalRequests)
		fmt.Printf("  成功请求: %d\n", result.SuccessRequests)
		fmt.Printf("  失败请求: %d\n", result.FailedRequests)
		fmt.Printf("  错误率: %.2f%%\n", result.ErrorRate)
		fmt.Printf("  平均延迟: %v\n", result.AverageLatency)
		fmt.Printf("  最小延迟: %v\n", result.MinLatency)
		fmt.Printf("  最大延迟: %v\n", result.MaxLatency)
		fmt.Printf("  RPS: %.2f\n", result.RequestsPerSecond)
	}

	// 输出JSON格式结果
	jsonResult, _ := json.MarshalIndent(pt.results, "", "  ")
	fmt.Printf("\n======= JSON结果 =======\n")
	fmt.Printf("%s\n", jsonResult)
}

// 预定义的测试配置
func getDefaultTestConfig() PerformanceTestConfig {
	return PerformanceTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 10,
		TestDuration:    2 * time.Minute,
		RequestInterval: 100 * time.Millisecond,
		TestEndpoints: []TestEndpoint{
			{
				Name:   "PlaybackInfo",
				Method: "POST",
				Path:   "/emby/Items/123456/PlaybackInfo",
				Headers: map[string]string{
					"Content-Type": "application/json",
					"X-Emby-Token": "your-api-key",
				},
				Body:   `{"DeviceProfile":{"MaxStreamingBitrate":120000000}}`,
				Weight: 3,
			},
			{
				Name:   "VideoStream",
				Method: "GET",
				Path:   "/emby/Videos/123456/stream?mediasourceid=mediasource_123456",
				Headers: map[string]string{
					"X-Emby-Token": "your-api-key",
				},
				Weight: 5,
			},
			{
				Name:   "CacheStats",
				Method: "GET",
				Path:   "/api/cache/stats",
				Weight: 1,
			},
		},
	}
}

// 比较测试：优化前后性能对比
func runComparisonTest() {
	fmt.Printf("======= 性能对比测试 =======\n")

	// 测试配置
	config := getDefaultTestConfig()
	config.TestDuration = 1 * time.Minute
	config.ConcurrentUsers = 5

	// 第一轮测试（模拟优化前）
	fmt.Printf("\n--- 第一轮测试（基准测试） ---\n")
	tester1 := NewPerformanceTester(config)
	tester1.RunTest()

	// 等待一段时间让缓存生效
	fmt.Printf("\n等待缓存预热...\n")
	time.Sleep(10 * time.Second)

	// 第二轮测试（模拟优化后）
	fmt.Printf("\n--- 第二轮测试（缓存优化后） ---\n")
	tester2 := NewPerformanceTester(config)
	tester2.RunTest()

	// 对比结果
	fmt.Printf("\n======= 性能提升对比 =======\n")
	for endpointName := range tester1.results {
		result1 := tester1.results[endpointName]
		result2 := tester2.results[endpointName]

		if result1.TotalRequests > 0 && result2.TotalRequests > 0 {
			latencyImprovement := float64(result1.AverageLatency-result2.AverageLatency) / float64(result1.AverageLatency) * 100
			rpsImprovement := (result2.RequestsPerSecond - result1.RequestsPerSecond) / result1.RequestsPerSecond * 100

			fmt.Printf("\n端点: %s\n", endpointName)
			fmt.Printf("  延迟改善: %.2f%% (%v -> %v)\n", latencyImprovement, result1.AverageLatency, result2.AverageLatency)
			fmt.Printf("  RPS提升: %.2f%% (%.2f -> %.2f)\n", rpsImprovement, result1.RequestsPerSecond, result2.RequestsPerSecond)
		}
	}
}

func main() {
	// 检查命令行参数
	if len(os.Args) > 1 && os.Args[1] == "compare" {
		runComparisonTest()
	} else {
		// 运行标准性能测试
		config := getDefaultTestConfig()
		tester := NewPerformanceTester(config)
		tester.RunTest()
	}
}
