package rclone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"MediaWarp/internal/logging"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
)

// RcloneClient 内部 rclone 客户端
type RcloneClient struct {
	initialized bool
}

// GlobalClient 全局 rclone 客户端实例
var GlobalClient *RcloneClient

// init 初始化全局客户端
func init() {
	GlobalClient = &RcloneClient{}
}

// Initialize 初始化 rclone 配置
func (c *RcloneClient) Initialize() error {
	if c.initialized {
		return nil
	}

	// 设置默认配置
	configfile.Install()

	c.initialized = true
	logging.Info("Rclone 内部客户端初始化完成")
	return nil
}

// GetDownloadURL 获取下载链接（内部实现）
func (c *RcloneClient) GetDownloadURL(ctx context.Context, remotePath, userAgent string) (string, error) {
	if !c.initialized {
		if err := c.Initialize(); err != nil {
			return "", fmt.Errorf("初始化 rclone 客户端失败: %w", err)
		}
	}

	logging.Info("Rclone 内部调用开始，路径:", remotePath)

	// 解析远程路径，例如 "115://path/to/file"
	colonIndex := strings.Index(remotePath, ":")
	if colonIndex == -1 {
		return "", fmt.Errorf("无效的远程路径格式: %s", remotePath)
	}

	remoteName := remotePath[:colonIndex] // 例如 "115"
	logging.Info("远程名称:", remoteName)

	// 创建文件系统实例
	fsInfo, err := fs.NewFs(ctx, remoteName+":")
	if err != nil {
		logging.Error("创建文件系统失败:", err)
		return "", fmt.Errorf("创建文件系统失败: %w", err)
	}

	logging.Info("文件系统创建成功，类型:", fsInfo.Name())

	// 检查是否支持 backend command
	if fsInfo.Features().Command == nil {
		logging.Warning("远程", remoteName, "不支持 backend command 功能")
		return "", fmt.Errorf("远程 %s 不支持 backend command 功能", remoteName)
	}

	// 准备参数
	args := []string{remotePath}
	opt := map[string]string{}

	// 保持原始 User-Agent，确保获取和播放时一致
	if userAgent != "" {
		opt["user-agent"] = userAgent
		logging.Info("使用原始 User-Agent:", userAgent)
	} else {
		// 只有在没有 User-Agent 时才使用默认值
		opt["user-agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
		logging.Info("使用默认 User-Agent")
	}

	logging.Info("调用 getDownloadURLCommand，参数:", args, "选项:", opt)

	// 调用 getDownloadURLCommand
	result, err := fsInfo.Features().Command(ctx, "get-download-url", args, opt)
	if err != nil {
		logging.Error("获取下载链接失败:", err)
		return "", fmt.Errorf("获取下载链接失败: %w", err)
	}

	// 解析结果
	if downloadURL, ok := result.(string); ok {
		logging.Info("成功获取下载链接:", downloadURL)
		return downloadURL, nil
	}

	return "", fmt.Errorf("无效的下载链接响应格式: %T", result)
}

// GetDownloadURLWithTimeout 带超时的获取下载链接
func (c *RcloneClient) GetDownloadURLWithTimeout(remotePath, userAgent string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return c.GetDownloadURL(ctx, remotePath, userAgent)
}

// 便利函数：直接调用全局客户端
func GetDownloadURL(remotePath, userAgent string) (string, error) {
	return GlobalClient.GetDownloadURLWithTimeout(remotePath, userAgent, 30*time.Second)
}
