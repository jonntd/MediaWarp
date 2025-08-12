package http

import (
	"context"
	"net/http"
	"time"
)

// SafeHTTPClient 安全的HTTP客户端配置
type SafeHTTPClient struct {
	client *http.Client
}

// NewSafeHTTPClient 创建安全的HTTP客户端
func NewSafeHTTPClient(timeout time.Duration) *SafeHTTPClient {
	return &SafeHTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
	}
}

// DoWithContext 执行带上下文的HTTP请求
func (c *SafeHTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return c.client.Do(req)
}

// Get 执行GET请求
func (c *SafeHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}

// Post 执行POST请求
func (c *SafeHTTPClient) Post(ctx context.Context, url, contentType string, body interface{}) (*http.Response, error) {
	// 这里可以根据需要实现POST请求
	return nil, nil
}

// 全局HTTP客户端实例
var (
	DefaultClient = NewSafeHTTPClient(30 * time.Second)
	FastClient    = NewSafeHTTPClient(10 * time.Second)
)
