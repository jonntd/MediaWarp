package handler

import (
	"MediaWarp/internal/cache"
	"MediaWarp/internal/config"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/process"
	"MediaWarp/internal/rclone"
	"MediaWarp/internal/security"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskManager struct {
	mu          sync.Mutex
	running     bool
	cond        *sync.Cond
	currentTask string    // 当前执行的任务名称
	startTime   time.Time // 任务开始时间
	queuedTasks []string  // 排队等待的任务
}

// 创建 TaskManager 的实例
func NewTaskManager() *TaskManager {
	tm := &TaskManager{}
	tm.cond = sync.NewCond(&tm.mu)
	return tm
}

// 任务执行函数，确保同一时间只有一个任务在运行
func (tm *TaskManager) RunTask(handler func()) {
	tm.RunTaskWithName("未知任务", handler)
}

// RunTaskWithName 带任务名称的任务执行函数
func (tm *TaskManager) RunTaskWithName(taskName string, handler func()) {
	go func() { // 任务执行在新的 Goroutine 中完全异步化
		tm.mu.Lock()

		// 如果有任务在运行，加入队列等待
		if tm.running {
			tm.queuedTasks = append(tm.queuedTasks, taskName)
			logging.Info("任务加入队列", "task", taskName, "queue_length", len(tm.queuedTasks))
		}

		for tm.running { // 如果有任务在运行，等待
			tm.cond.Wait()
		}

		// 从队列中移除当前任务
		if len(tm.queuedTasks) > 0 && tm.queuedTasks[0] == taskName {
			tm.queuedTasks = tm.queuedTasks[1:]
		}

		// 开始执行任务
		tm.running = true
		tm.currentTask = taskName
		tm.startTime = time.Now()
		tm.mu.Unlock()

		logging.Info("开始执行任务", "task", taskName, "start_time", tm.startTime)

		handler() // 执行任务

		logging.Info("任务执行完成", "task", taskName, "duration", time.Since(tm.startTime))

		time.Sleep(30 * time.Second) // 任务结束后等待 30 秒

		tm.mu.Lock()
		tm.running = false
		tm.currentTask = ""
		tm.startTime = time.Time{}
		tm.cond.Signal() // 通知下一个任务可以开始执行
		tm.mu.Unlock()

		logging.Info("任务管理器空闲", "next_queue_length", len(tm.queuedTasks))
	}()
}

// TaskManagerStatus 任务管理器状态
type TaskManagerStatus struct {
	Running     bool     `json:"running"`
	CurrentTask string   `json:"current_task"`
	StartTime   string   `json:"start_time,omitempty"`
	Duration    string   `json:"duration,omitempty"`
	QueuedTasks []string `json:"queued_tasks"`
	QueueLength int      `json:"queue_length"`
}

// GetStatus 获取任务管理器状态
func (tm *TaskManager) GetStatus() TaskManagerStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	status := TaskManagerStatus{
		Running:     tm.running,
		CurrentTask: tm.currentTask,
		QueuedTasks: make([]string, len(tm.queuedTasks)),
		QueueLength: len(tm.queuedTasks),
	}

	// 复制队列任务列表
	copy(status.QueuedTasks, tm.queuedTasks)

	// 如果有任务在运行，计算运行时间
	if tm.running && !tm.startTime.IsZero() {
		status.StartTime = tm.startTime.Format("2006-01-02 15:04:05")
		status.Duration = time.Since(tm.startTime).Round(time.Second).String()
	}

	return status
}

var taskManager = NewTaskManager() // 定义全局任务管理器，只允许一个任务同时运行

func MediaFileSyncHandler(ctx *gin.Context) {
	fullPath := ctx.Param("path")
	serverAddr := ctx.GetHeader("X-Alist-Server")
	prefixPath := ctx.GetHeader("X-Prefix-Path")

	// 安全验证
	if err := security.ValidatePath(fullPath); err != nil {
		logging.Warning("无效的路径参数:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "error",
			"error":   "Invalid path parameter",
		})
		return
	}

	if serverAddr != "" {
		if cleanAddr, err := security.SanitizeServerAddr(serverAddr); err != nil {
			logging.Warning("无效的服务器地址:", err)
			ctx.JSON(http.StatusBadRequest, gin.H{
				"message": "error",
				"error":   "Invalid server address",
			})
			return
		} else {
			serverAddr = cleanAddr
		}
	}

	sourceDir := serverAddr + ":" + fullPath

	logging.Infof("sourceDirt: %s", sourceDir)
	logging.Infof("remoteDest: %s", prefixPath)
	logging.Infof("serverAddr:  %s", serverAddr)
	logging.Infof("fullPath:  %s", fullPath)
	// if ctx.Request.Header.Get("Emby-Token") != config.ApiKey {
	// 	ctx.JSON(401, gin.H{
	// 		"message": "error",
	// 		"error":   "Invalid Emby-Token",
	// 	})
	// 	return
	// }

	ctx.JSON(200, gin.H{
		"message": "success",
		"path":    fullPath})

	// 获取服务器地址，默认使用第一个
	if serverAddr == "" && len(config.MediaSync) > 0 {
		serverAddr = config.MediaSync[0].Name
	}

	// 查找对应的服务器配置
	var serverConfig *config.MediaSyncServerSetting
	for _, server := range config.MediaSync {
		if server.Name == serverAddr {
			serverConfig = &server
			break
		}
	}

	if serverConfig == nil {
		logging.Error("未找到服务器配置:", serverAddr)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "error",
			"error":   "未找到服务器配置: " + serverAddr,
		})
		return
	}

	// 使用配置中的本地路径
	if prefixPath == "" {
		prefixPath = serverConfig.LocalPath
	}

	taskManager.RunTask(func() {
		syncAndCreateEmptyFiles(serverConfig.Remote+sourceDir, prefixPath)
	})
}
func syncAndCreateEmptyFiles(sourceDir, remoteDest string) {
	colonIndex := strings.Index(sourceDir, ":")

	// 使用 sync 命令进行同步
	err := runBackendMediaSync(sourceDir, remoteDest, colonIndex)
	if err != nil {
		fmt.Printf("Error during sync: %v\n", err)
	}

	scanMediaLibrary()
}

func runBackendMediaSync(sourceDir, remoteDest string, colonIndex int) error {
	// 构建目标路径，参考 runRcloneSync 的路径处理方式
	targetPath := filepath.Join(remoteDest, sourceDir[colonIndex+1:])

	logging.Info("使用内部rclone backend调用")
	logging.Info("sourceDir:", sourceDir)
	logging.Info("remoteDest:", remoteDest)
	logging.Info("targetPath:", targetPath)

	// 从sourceDir中提取远程名称（如 "115:"）
	remoteName := sourceDir[:colonIndex+1]
	remotePath := sourceDir[colonIndex+1:]

	logging.Info("remoteName:", remoteName)
	logging.Info("remotePath:", remotePath)

	// 准备backend选项
	options := map[string]string{
		"min-size":    "100M",
		"strm-format": "",
		"sync-delete": "",
		"vv":          "", // 详细日志输出
	}

	// 使用内部rclone库调用backend命令
	err := rclone.Backend("media-sync", remoteName, remotePath, targetPath, options)
	if err != nil {
		logging.Error("内部rclone backend调用失败:", err)
		return fmt.Errorf("内部rclone backend调用失败: %w", err)
	}

	logging.Info("内部rclone backend调用成功")
	return nil
}

type Task struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

func scanMediaLibrary() {
	// 设置 Emby 服务器信息
	// embyServer := config.Origin
	// apiKey := config.ApiKey // 替换为实际的 API 密钥
	// 构建获取任务列表的 URL

	embyServer := fmt.Sprintf("http://0.0.0.0:%d", config.Port)
	apiKey := config.MediaServer.AUTH
	url := fmt.Sprintf("%s/emby/ScheduledTasks?api_key=%s", embyServer, apiKey)

	// 发送 GET 请求获取任务列表
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	// 解析 JSON 响应
	var tasks []Task
	if err := json.Unmarshal(body, &tasks); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// 查找 "Scan media library" 任务的 ID
	var scanTaskId string
	for _, task := range tasks {
		if task.Name == "Scan media library" {
			scanTaskId = task.Id
			break
		}
	}

	if scanTaskId == "" {
		fmt.Println("Scan media library task not found")
		return
	}

	// 构建执行任务的 URL
	runUrl := fmt.Sprintf("%s/emby/ScheduledTasks/Running/%s?api_key=%s", embyServer, scanTaskId, apiKey)

	// 发送 POST 请求执行任务
	runReq, err := http.NewRequest("POST", runUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	runResp, err := http.DefaultClient.Do(runReq)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer runResp.Body.Close()
	if runResp.StatusCode == http.StatusNoContent {
		fmt.Println("Scan Media Library task executed successfully")
	}

}

// Middleware to check API Key
func apiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != config.MediaServer.AUTH {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid API Key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// Handle the API Key verification
func verifyAPIKey(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == config.MediaServer.AUTH {
		c.JSON(http.StatusOK, gin.H{"message": "API Key is valid"})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API Key"})
	}
}

// IndexHandler renders the main page with the folder structure
func SyncfolderHandler(ctx *gin.Context) {
	apiKey := ctx.Query("apikey")
	path := ctx.Query("path")
	serverAddr := ctx.Query("server")
	prefixPath := ctx.Query("prefix")

	if path == "" {
		path = ""
	}

	// 获取服务器地址，默认使用第一个
	if serverAddr == "" && len(config.MediaSync) > 0 {
		serverAddr = config.MediaSync[0].Name
	}

	// 查找对应的服务器配置
	var serverConfig *config.MediaSyncServerSetting
	for _, server := range config.MediaSync {
		if server.Name == serverAddr {
			serverConfig = &server
			break
		}
	}

	if serverConfig == nil {
		ctx.HTML(http.StatusOK, "syncFolder.html", gin.H{
			"Path":          path,
			"Folders":       []string{},
			"Servers":       config.MediaSync,
			"CurrentServer": serverAddr,
			"PrefixList":    []string{},
			"CurrentPrefix": "",
			"Error":         "未找到服务器配置: " + serverAddr,
		})
		return
	}

	// 使用配置中的本地路径
	if prefixPath == "" {
		prefixPath = serverConfig.LocalPath
	}

	var folders []string
	if apiKey == config.MediaServer.AUTH {
		// 安全验证参数
		if err := security.ValidatePath(path); err != nil {
			logging.Warning("无效的路径参数:", err)
			ctx.String(http.StatusBadRequest, "Invalid path parameter")
			return
		}

		if cleanAddr, err := security.SanitizeServerAddr(serverAddr); err != nil {
			logging.Warning("无效的服务器地址:", err)
			ctx.String(http.StatusBadRequest, "Invalid server address")
			return
		} else {
			serverAddr = cleanAddr
		}

		// 首先尝试从缓存获取
		if cachedFolders, found := cache.GlobalFolderCache.Get(serverAddr, path); found {
			logging.Info("使用缓存的文件夹列表", "server", serverAddr, "path", path, "count", len(cachedFolders))
			folders = cachedFolders
		} else {
			// 缓存未命中，执行rclone命令
			logging.Info("缓存未命中，执行rclone命令", "server", serverAddr, "path", path)

			// 根据路径复杂度动态调整超时时间
			timeout := 30 * time.Second
			if strings.Contains(path, "电影") || strings.Contains(path, "video") || strings.Contains(path, "movie") {
				timeout = 60 * time.Second // 视频目录通常文件较多，增加超时时间
				logging.Info("检测到视频目录，增加超时时间", "path", path, "timeout", timeout)
			}

			// 使用安全的进程管理执行命令
			ctx_timeout, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			rcloneCmd := serverAddr + ":" + path
			output, err := process.RunWithOutput(ctx_timeout, timeout, "rclone", "lsf", rcloneCmd, "--dirs-only")
			if err != nil {
				logging.Error("执行rclone命令失败", "server", serverAddr, "path", path, "command", rcloneCmd, "error", err)

				// 根据错误类型提供更具体的错误信息
				var errorMsg string
				if strings.Contains(err.Error(), "exit status 3") {
					errorMsg = fmt.Sprintf("路径不存在或配置错误: %s", rcloneCmd)
				} else if strings.Contains(err.Error(), "exit status 1") {
					errorMsg = fmt.Sprintf("认证失败或权限不足: %s", rcloneCmd)
				} else if strings.Contains(err.Error(), "signal: killed") || strings.Contains(err.Error(), "context deadline exceeded") {
					errorMsg = fmt.Sprintf("目录扫描超时，文件夹可能包含大量内容: %s (超时时间: %v)", rcloneCmd, timeout)
					logging.Warning("rclone命令超时", "server", serverAddr, "path", path, "timeout", timeout, "suggestion", "考虑增加超时时间或优化目录结构")
				} else {
					errorMsg = fmt.Sprintf("rclone命令执行失败: %s", err.Error())
				}

				// 记录错误但不缓存失败结果
				ctx.String(http.StatusInternalServerError, errorMsg)
				return
			}

			// Split the output into lines
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			// Create a slice of folders
			for _, line := range lines {
				if line != "" {
					folders = append(folders, line)
				}
			}

			// 将结果保存到缓存
			cache.GlobalFolderCache.Set(serverAddr, path, folders)
			logging.Info("文件夹列表已缓存", "server", serverAddr, "path", path, "count", len(folders))
		}
	}

	// 使用gin的HTML方法渲染模板
	ctx.HTML(http.StatusOK, "syncFolder.html", gin.H{
		"Path":          path,
		"Folders":       folders,
		"Servers":       config.MediaSync,
		"CurrentServer": serverAddr,
		"PrefixList":    []string{prefixPath}, // 当前使用的前缀路径
		"CurrentPrefix": prefixPath,
	})
}

// 缓存管理API处理函数
func cacheStatsHandler(ctx *gin.Context) {
	stats := cache.GlobalFolderCache.GetStats()
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Cache statistics",
		"stats":   stats,
	})
}

func clearCacheHandler(ctx *gin.Context) {
	server := ctx.Query("server")
	if server != "" {
		// 清除指定服务器的缓存
		cache.GlobalFolderCache.ClearByServer(server)
		logging.Info("已清除指定服务器的缓存", "server", server)
		ctx.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Cache cleared for server: %s", server),
		})
	} else {
		// 清除所有缓存
		cache.GlobalFolderCache.Clear()
		logging.Info("已清除所有缓存")
		ctx.JSON(http.StatusOK, gin.H{
			"message": "All cache cleared",
		})
	}
}

func exportCacheHandler(ctx *gin.Context) {
	data, err := cache.GlobalFolderCache.Export()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export cache",
		})
		return
	}

	ctx.Header("Content-Type", "application/json")
	ctx.Header("Content-Disposition", "attachment; filename=folder_cache.json")
	ctx.Data(http.StatusOK, "application/json", data)
}

func SyncFilesRouter(router *gin.Engine) {
	// API to verify API Key
	router.POST("/verify", verifyAPIKey)

	// Routes with API Key auth
	router.GET("/syncfolder", SyncfolderHandler)
	router.POST("/Sync/*path", apiKeyAuth(), MediaFileSyncHandler)

	// 缓存管理API (需要API Key认证)
	router.GET("/cache/stats", apiKeyAuth(), cacheStatsHandler)
	router.POST("/cache/clear", apiKeyAuth(), clearCacheHandler)
	router.GET("/cache/export", apiKeyAuth(), exportCacheHandler)
}
