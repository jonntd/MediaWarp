package handler

import (
	"MediaWarp/internal/config"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/process"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

var (
	TaskCron      *cron.Cron
	taskIDs       map[string]cron.EntryID
	taskSchedules map[string]string
	taskFunctions map[string]string
	taskInfos     map[string]*TaskInfo // 新增：存储完整的任务信息
	mu            sync.Mutex
)

type TaskInfo struct {
	Name        string    `json:"name"`
	Schedule    string    `json:"schedule"`
	Function    string    `json:"function"`
	NextRun     string    `json:"next_run,omitempty"`
	LastRun     string    `json:"last_run,omitempty"`
	Status      string    `json:"status,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	// 新增字段，使用omitempty确保向后兼容
	TaskType     string            `json:"task_type,omitempty"`     // "predefined" 或 "custom_sync"
	CustomParams *CustomSyncParams `json:"custom_params,omitempty"` // 自定义同步参数
}

// CustomSyncParams 自定义同步任务参数
type CustomSyncParams struct {
	SourcePath  string   `json:"source_path"`  // 源路径，如: "115:/bbb/"
	TargetPath  string   `json:"target_path"`  // 目标路径，如: "/Users/jonntd/data/media-server/media115/bbb"
	SyncOptions []string `json:"sync_options"` // 同步选项，如: ["min-size=100M", "strm-format", "sync-delete"]
	RclonePath  string   `json:"rclone_path"`  // rclone可执行文件路径，默认"rclone"
}

type TaskExecution struct {
	TaskName  string    `json:"task_name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Status    string    `json:"status"` // running, completed, failed
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func init() {
	TaskCron = cron.New(cron.WithSeconds())
	taskIDs = make(map[string]cron.EntryID)
	taskSchedules = make(map[string]string)
	taskFunctions = make(map[string]string)
	taskInfos = make(map[string]*TaskInfo) // 初始化任务信息映射
	loadTasksFromFile()
}

// 清理日志文件
func cleanupLogs() {
	logging.Info("开始清理日志文件...")

	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logging.Warning("日志目录不存在:", logDir)
		return
	}

	// 删除7天前的日志文件
	cutoffTime := time.Now().AddDate(0, 0, -7)

	err := filepath.Walk(logDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				logging.Error("删除日志文件失败:", path, err)
			} else {
				logging.Info("已删除过期日志文件:", path)
			}
		}
		return nil
	})

	if err != nil {
		logging.Error("清理日志文件时出错:", err)
	} else {
		logging.Info("日志文件清理完成")
	}
}

// 同步媒体库
func syncMediaLibrary() {
	logging.Info("开始同步媒体库...")

	// 调用现有的同步功能（需要根据实际配置调整）
	logging.Info("媒体库同步功能需要根据实际配置进行调整")
	// 示例：if err := runRcloneSync("/media", "remote:/media", 3); err != nil {
	//     logging.Error("媒体库同步失败:", err)
	// } else {
	//     logging.Info("媒体库同步完成")
	// }
}

// 健康检查任务
func healthCheckTask() {
	logging.Info("执行系统健康检查...")

	// 检查磁盘空间
	cmd := exec.Command("df", "-h")
	output, err := cmd.Output()
	if err != nil {
		logging.Error("磁盘空间检查失败:", err)
	} else {
		logging.Info("磁盘空间状态:\n", string(output))
	}

	// 检查内存使用
	cmd = exec.Command("free", "-h")
	output, err = cmd.Output()
	if err != nil {
		logging.Warning("内存检查失败 (可能不是Linux系统):", err)
	} else {
		logging.Info("内存使用状态:\n", string(output))
	}
}

// 备份配置文件
func backupConfig() {
	logging.Info("开始备份配置文件...")

	configFile := "config/config.yaml"
	backupDir := "backups"

	// 创建备份目录
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logging.Error("创建备份目录失败:", err)
		return
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("config_%s.yaml", timestamp))

	// 复制配置文件
	cmd := exec.Command("cp", configFile, backupFile)
	if err := cmd.Run(); err != nil {
		logging.Error("备份配置文件失败:", err)
	} else {
		logging.Info("配置文件备份完成:", backupFile)
	}
}

// 重启服务任务
func restartService() {
	logging.Warning("执行服务重启任务...")

	// 这里可以添加重启逻辑
	// 注意：实际重启需要谨慎处理
	logging.Info("服务重启任务完成")
}

// validateCustomSyncParams 验证自定义同步参数
func validateCustomSyncParams(params *CustomSyncParams) error {
	if params == nil {
		return fmt.Errorf("自定义同步参数不能为空")
	}

	// 验证源路径
	if params.SourcePath == "" {
		return fmt.Errorf("源路径不能为空")
	}
	if !strings.Contains(params.SourcePath, ":") {
		return fmt.Errorf("源路径格式错误，应为 'remote:path' 格式")
	}

	// 验证目标路径
	if params.TargetPath == "" {
		return fmt.Errorf("目标路径不能为空")
	}
	if !filepath.IsAbs(params.TargetPath) {
		return fmt.Errorf("目标路径必须是绝对路径")
	}

	// 验证rclone路径
	if params.RclonePath == "" {
		params.RclonePath = "rclone" // 设置默认值
	}

	// 验证同步选项
	for _, option := range params.SyncOptions {
		if strings.TrimSpace(option) == "" {
			return fmt.Errorf("同步选项不能为空")
		}
		// 基本的选项格式验证
		if !strings.Contains(option, "=") && !strings.HasPrefix(option, "-") {
			// 允许 key=value 格式或 -flag 格式
			if option != "strm-format" && option != "sync-delete" {
				return fmt.Errorf("同步选项格式错误: %s", option)
			}
		}
	}

	return nil
}

// executeCustomSync 执行自定义同步任务
func executeCustomSync(params *CustomSyncParams) error {
	// 输入验证
	if err := validateCustomSyncParams(params); err != nil {
		logging.Error("自定义同步参数验证失败", "error", err)
		return err
	}

	logging.Info("开始执行自定义同步任务",
		"source", params.SourcePath,
		"target", params.TargetPath,
		"options", params.SyncOptions)

	// 构建rclone命令参数
	args := []string{"backend", "media-sync", params.SourcePath, params.TargetPath}

	// 添加同步选项
	for _, option := range params.SyncOptions {
		args = append(args, "-o", option)
	}

	// 添加详细输出
	args = append(args, "-vv")

	// 创建带超时的上下文（30分钟）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	logging.Info("执行rclone命令", "command", params.RclonePath, "args", args)

	// 使用安全进程管理器执行命令
	err := process.RunWithTimeout(ctx, 30*time.Minute, params.RclonePath, args...)
	if err != nil {
		logging.Error("自定义同步任务执行失败",
			"source", params.SourcePath,
			"target", params.TargetPath,
			"error", err)
		return fmt.Errorf("自定义同步执行失败: %v", err)
	}

	logging.Info("自定义同步任务执行成功",
		"source", params.SourcePath,
		"target", params.TargetPath)

	// 同步完成后，通知Emby进行媒体库刮削
	logging.Info("开始通知Emby进行媒体库刮削")
	scanMediaLibrary()

	return nil
}

// createCustomSyncTaskFunc 创建自定义同步任务函数，使用taskManager确保同一时间只有一个任务运行
func createCustomSyncTaskFunc(taskName string, params *CustomSyncParams) func() {
	return func() {
		// 使用taskManager确保同一时间只有一个同步任务运行
		executeCustomSyncWithTaskManager(taskName, params)
	}
}

// executeCustomSyncWithTaskManager 使用任务管理器执行自定义同步
func executeCustomSyncWithTaskManager(taskName string, params *CustomSyncParams) {
	logging.Info("准备执行自定义同步任务", "task", taskName, "source", params.SourcePath, "target", params.TargetPath)

	// 使用全局taskManager确保同一时间只有一个同步任务运行
	// 这与syncfiles.go中的同步任务保持一致的执行顺序
	taskManager.RunTaskWithName(taskName, func() {
		logging.Info("开始执行自定义同步任务", "task", taskName, "source", params.SourcePath, "target", params.TargetPath)

		if err := executeCustomSync(params); err != nil {
			logging.Error("自定义同步任务失败", "task", taskName, "error", err)
		} else {
			logging.Info("自定义同步任务完成", "task", taskName, "source", params.SourcePath, "target", params.TargetPath)
		}
	})
}

// 自定义函数（保持向后兼容）
func customFunction1() { cleanupLogs() }
func customFunction2() { syncMediaLibrary() }
func customFunction3() { healthCheckTask() }

// 包装函数，确保所有任务都通过taskManager执行
func wrapWithTaskManager(taskName string, taskFunc func()) func() {
	return func() {
		taskManager.RunTaskWithName(taskName, taskFunc)
	}
}

var taskFunctionsMap = map[string]func(){
	"cleanup_logs":    wrapWithTaskManager("清理日志", cleanupLogs),
	"sync_media":      wrapWithTaskManager("同步媒体库", syncMediaLibrary),
	"health_check":    wrapWithTaskManager("健康检查", healthCheckTask),
	"backup_config":   wrapWithTaskManager("备份配置", backupConfig),
	"restart_service": wrapWithTaskManager("重启服务", restartService),
	// 保持向后兼容
	"func1": wrapWithTaskManager("自定义功能1", customFunction1),
	"func2": wrapWithTaskManager("自定义功能2", customFunction2),
	"func3": wrapWithTaskManager("自定义功能3", customFunction3),
}

// 获取任务函数描述
var taskDescriptions = map[string]string{
	"cleanup_logs":    "清理7天前的日志文件",
	"sync_media":      "同步媒体库到远程存储",
	"health_check":    "执行系统健康检查",
	"backup_config":   "备份配置文件",
	"restart_service": "重启服务 (谨慎使用)",
	"func1":           "清理日志文件 (兼容)",
	"func2":           "同步媒体库 (兼容)",
	"func3":           "健康检查 (兼容)",
}

func addTask(c *gin.Context) {
	var taskInfo TaskInfo
	if err := c.BindJSON(&taskInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task data"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := taskIDs[taskInfo.Name]; exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task already exists"})
		return
	}

	// 设置默认值
	if taskInfo.TaskType == "" {
		taskInfo.TaskType = "predefined"
	}
	if taskInfo.CreatedAt.IsZero() {
		taskInfo.CreatedAt = time.Now()
	}

	// 根据任务类型验证和处理
	var taskFunc func()
	if taskInfo.TaskType == "predefined" {
		// 预定义任务
		predefinedFunc, exists := taskFunctionsMap[taskInfo.Function]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name"})
			return
		}
		taskFunc = predefinedFunc
	} else if taskInfo.TaskType == "custom_sync" {
		// 自定义同步任务
		if taskInfo.CustomParams == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Custom sync task requires custom_params"})
			return
		}

		// 验证自定义同步参数
		if err := validateCustomSyncParams(taskInfo.CustomParams); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom sync params: %v", err)})
			return
		}

		// 创建自定义同步任务函数
		taskFunc = createCustomSyncTaskFunc(taskInfo.Name, taskInfo.CustomParams)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task type"})
		return
	}

	go taskFunc() // 可选：立即执行一次
	entryID, err := TaskCron.AddFunc(taskInfo.Schedule, taskFunc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
		return
	}

	// 保存到各种映射中
	taskSchedules[taskInfo.Name] = taskInfo.Schedule
	taskIDs[taskInfo.Name] = entryID
	taskFunctions[taskInfo.Name] = taskInfo.Function

	// 保存完整的任务信息
	taskInfos[taskInfo.Name] = &taskInfo

	saveTasksToFile()

	c.JSON(http.StatusOK, gin.H{"status": "Task added", "task_name": taskInfo.Name})
}

func listTasks(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	tasks := make([]TaskInfo, 0, len(taskIDs))
	for name, entryID := range taskIDs {
		entry := TaskCron.Entry(entryID)

		// 获取完整的任务信息
		taskInfo := taskInfos[name]
		if taskInfo == nil {
			// 如果没有完整信息，创建基本信息（向后兼容）
			functionName := taskFunctions[name]
			taskInfo = &TaskInfo{
				Name:        name,
				Schedule:    taskSchedules[name],
				Function:    functionName,
				TaskType:    "predefined",
				Description: taskDescriptions[functionName],
				Status:      "active",
				CreatedAt:   time.Now(),
			}
		}

		// 更新运行时信息
		taskInfo.NextRun = entry.Next.String()
		taskInfo.Status = "active"

		// 设置描述（如果没有）
		if taskInfo.Description == "" && taskInfo.TaskType == "predefined" {
			taskInfo.Description = taskDescriptions[taskInfo.Function]
		}

		tasks = append(tasks, *taskInfo)
	}

	c.JSON(http.StatusOK, tasks)
}

// getTaskDetail 获取单个任务的详细信息
func getTaskDetail(c *gin.Context) {
	taskName := c.Param("name")

	mu.Lock()
	defer mu.Unlock()

	// 检查任务是否存在
	entryID, exists := taskIDs[taskName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// 获取cron任务信息
	entry := TaskCron.Entry(entryID)

	// 获取完整的任务信息
	taskInfo := taskInfos[taskName]
	if taskInfo == nil {
		// 如果没有完整信息，创建基本信息（向后兼容）
		functionName := taskFunctions[taskName]
		taskInfo = &TaskInfo{
			Name:        taskName,
			Schedule:    taskSchedules[taskName],
			Function:    functionName,
			TaskType:    "predefined",
			Description: taskDescriptions[functionName],
			Status:      "active",
			CreatedAt:   time.Now(),
		}
	}

	// 更新运行时信息
	taskInfo.NextRun = entry.Next.String()
	taskInfo.Status = "active"

	// 设置描述（如果没有）
	if taskInfo.Description == "" && taskInfo.TaskType == "predefined" {
		taskInfo.Description = taskDescriptions[taskInfo.Function]
	}

	c.JSON(http.StatusOK, *taskInfo)
}

// 获取可用的任务函数列表
func getTaskFunctions(c *gin.Context) {
	functions := make([]map[string]string, 0, len(taskFunctionsMap)+1)

	// 添加预定义函数
	for key, desc := range taskDescriptions {
		functions = append(functions, map[string]string{
			"key":         key,
			"description": desc,
		})
	}

	// 添加自定义同步选项
	functions = append(functions, map[string]string{
		"key":         "custom_sync",
		"description": "自定义同步任务 - 配置rclone同步参数",
	})

	c.JSON(http.StatusOK, functions)
}

func deleteTask(c *gin.Context) {
	taskName := c.Param("name")
	fmt.Printf("taskName: %s\n", taskName)
	mu.Lock()
	defer mu.Unlock()

	entryID, exists := taskIDs[taskName]
	if exists {
		TaskCron.Remove(entryID)
		delete(taskIDs, taskName)
		delete(taskSchedules, taskName)
		delete(taskFunctions, taskName) // 删除对应的 function 名称
		delete(taskInfos, taskName)     // 删除完整的任务信息
		saveTasksToFile()               // 删除任务后保存到文件
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Task deleted", "task_name": taskName})
}

// updateTask 更新现有任务（通过删除旧任务并创建新任务实现）
func updateTask(c *gin.Context) {
	taskName := c.Param("name")

	var newTaskInfo TaskInfo
	if err := c.BindJSON(&newTaskInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task data"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 检查任务是否存在
	entryID, exists := taskIDs[taskName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// 保存旧任务信息（用于回滚）
	oldTaskInfo := taskInfos[taskName]

	// 删除旧任务
	TaskCron.Remove(entryID)
	delete(taskIDs, taskName)
	delete(taskSchedules, taskName)
	delete(taskFunctions, taskName)
	delete(taskInfos, taskName)

	// 设置新任务的默认值
	if newTaskInfo.TaskType == "" {
		newTaskInfo.TaskType = "predefined"
	}
	if newTaskInfo.CreatedAt.IsZero() {
		// 保持原有的创建时间，如果有的话
		if oldTaskInfo != nil && !oldTaskInfo.CreatedAt.IsZero() {
			newTaskInfo.CreatedAt = oldTaskInfo.CreatedAt
		} else {
			newTaskInfo.CreatedAt = time.Now()
		}
	}

	// 根据任务类型验证和处理
	var taskFunc func()
	if newTaskInfo.TaskType == "predefined" {
		// 预定义任务
		predefinedFunc, exists := taskFunctionsMap[newTaskInfo.Function]
		if !exists {
			// 回滚：重新创建旧任务
			rollbackTask(taskName, oldTaskInfo)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name"})
			return
		}
		taskFunc = predefinedFunc
	} else if newTaskInfo.TaskType == "custom_sync" {
		// 自定义同步任务
		if newTaskInfo.CustomParams == nil {
			rollbackTask(taskName, oldTaskInfo)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Custom sync task requires custom_params"})
			return
		}

		// 验证自定义同步参数
		if err := validateCustomSyncParams(newTaskInfo.CustomParams); err != nil {
			rollbackTask(taskName, oldTaskInfo)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom sync params: %v", err)})
			return
		}

		// 创建自定义同步任务函数
		taskFunc = createCustomSyncTaskFunc(newTaskInfo.Name, newTaskInfo.CustomParams)
	} else {
		rollbackTask(taskName, oldTaskInfo)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task type"})
		return
	}

	// 创建新任务
	newEntryID, err := TaskCron.AddFunc(newTaskInfo.Schedule, taskFunc)
	if err != nil {
		// 回滚：重新创建旧任务
		rollbackTask(taskName, oldTaskInfo)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
		return
	}

	// 保存新任务信息
	taskSchedules[taskName] = newTaskInfo.Schedule
	taskIDs[taskName] = newEntryID
	taskFunctions[taskName] = newTaskInfo.Function
	taskInfos[taskName] = &newTaskInfo

	saveTasksToFile()

	c.JSON(http.StatusOK, gin.H{"status": "Task updated", "task_name": taskName})
}

// rollbackTask 回滚任务（用于更新失败时恢复）
func rollbackTask(taskName string, oldTaskInfo *TaskInfo) {
	if oldTaskInfo == nil {
		return
	}

	// 根据任务类型重新创建任务函数
	var taskFunc func()
	if oldTaskInfo.TaskType == "predefined" {
		if predefinedFunc, exists := taskFunctionsMap[oldTaskInfo.Function]; exists {
			taskFunc = predefinedFunc
		} else {
			return // 无法回滚
		}
	} else if oldTaskInfo.TaskType == "custom_sync" && oldTaskInfo.CustomParams != nil {
		taskFunc = createCustomSyncTaskFunc(oldTaskInfo.Name, oldTaskInfo.CustomParams)
	} else {
		return // 无法回滚
	}

	// 重新创建任务
	if entryID, err := TaskCron.AddFunc(oldTaskInfo.Schedule, taskFunc); err == nil {
		taskIDs[taskName] = entryID
		taskSchedules[taskName] = oldTaskInfo.Schedule
		taskFunctions[taskName] = oldTaskInfo.Function
		taskInfos[taskName] = oldTaskInfo
	}
}

// addCustomSyncTask 专门用于创建自定义同步任务的API端点
func addCustomSyncTask(c *gin.Context) {
	var customSyncRequest struct {
		Name         string            `json:"name" binding:"required"`
		Schedule     string            `json:"schedule" binding:"required"`
		Description  string            `json:"description"`
		CustomParams *CustomSyncParams `json:"custom_params" binding:"required"`
	}

	if err := c.BindJSON(&customSyncRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 检查任务名称是否已存在
	if _, exists := taskIDs[customSyncRequest.Name]; exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task already exists"})
		return
	}

	// 验证自定义同步参数
	if err := validateCustomSyncParams(customSyncRequest.CustomParams); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom sync params: %v", err)})
		return
	}

	// 创建TaskInfo
	taskInfo := TaskInfo{
		Name:         customSyncRequest.Name,
		Schedule:     customSyncRequest.Schedule,
		Function:     "custom_sync",
		TaskType:     "custom_sync",
		CustomParams: customSyncRequest.CustomParams,
		Description:  customSyncRequest.Description,
		CreatedAt:    time.Now(),
	}

	// 如果没有提供描述，生成默认描述
	if taskInfo.Description == "" {
		taskInfo.Description = fmt.Sprintf("自定义同步任务: %s -> %s",
			customSyncRequest.CustomParams.SourcePath,
			customSyncRequest.CustomParams.TargetPath)
	}

	// 创建自定义同步任务函数
	taskFunc := createCustomSyncTaskFunc(taskInfo.Name, customSyncRequest.CustomParams)

	// 添加到cron调度器
	entryID, err := TaskCron.AddFunc(taskInfo.Schedule, taskFunc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
		return
	}

	// 保存到各种映射中
	taskSchedules[taskInfo.Name] = taskInfo.Schedule
	taskIDs[taskInfo.Name] = entryID
	taskFunctions[taskInfo.Name] = taskInfo.Function
	taskInfos[taskInfo.Name] = &taskInfo

	saveTasksToFile()

	c.JSON(http.StatusOK, gin.H{
		"status":    "Custom sync task created",
		"task_name": taskInfo.Name,
		"task_type": taskInfo.TaskType,
		"source":    customSyncRequest.CustomParams.SourcePath,
		"target":    customSyncRequest.CustomParams.TargetPath,
	})
}

// getTaskManagerStatus 获取任务管理器状态
func getTaskManagerStatus(c *gin.Context) {
	status := taskManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

func saveTasksToFile() {
	file, err := os.Create("tasks.json")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	tasks := make([]TaskInfo, 0, len(taskIDs))
	for name, entryID := range taskIDs {
		entry := TaskCron.Entry(entryID)

		// 获取完整的任务信息
		taskInfo := taskInfos[name]
		if taskInfo == nil {
			// 如果没有完整信息，创建基本信息（向后兼容）
			taskInfo = &TaskInfo{
				Name:     name,
				Schedule: taskSchedules[name],
				Function: taskFunctions[name],
				TaskType: "predefined", // 默认为预定义任务
			}
		}

		// 更新运行时信息
		taskInfo.NextRun = entry.Next.String()
		tasks = append(tasks, *taskInfo)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // 格式化JSON输出
	if err := encoder.Encode(tasks); err != nil {
		fmt.Printf("Error encoding tasks to JSON: %v\n", err)
	}
	fmt.Printf("saveTasksToFile: saved %d tasks\n", len(tasks))
}

func loadTasksFromFile() {
	file, err := os.Open("tasks.json")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("tasks.json file does not exist. Skipping load.\n")
			return
		}
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	var tasks []TaskInfo
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&tasks); err != nil {
		fmt.Printf("Error decoding JSON: %v\n", err)
		return
	}

	loadedCount := 0
	for _, task := range tasks {
		// 数据迁移：为旧任务设置默认TaskType
		if task.TaskType == "" {
			task.TaskType = "predefined"
		}

		// 设置创建时间（如果没有）
		if task.CreatedAt.IsZero() {
			task.CreatedAt = time.Now()
		}

		// 根据任务类型处理不同的执行逻辑
		var taskFunc func()
		if task.TaskType == "predefined" {
			// 预定义任务
			predefinedFunc, exists := taskFunctionsMap[task.Function]
			if !exists {
				fmt.Printf("Invalid predefined function name: %s\n", task.Function)
				continue
			}
			taskFunc = predefinedFunc
		} else if task.TaskType == "custom_sync" {
			// 自定义同步任务
			if task.CustomParams == nil {
				fmt.Printf("Custom sync task %s has no custom_params, skipping\n", task.Name)
				continue
			}

			// 验证自定义同步参数
			if err := validateCustomSyncParams(task.CustomParams); err != nil {
				fmt.Printf("Custom sync task %s has invalid params: %v, skipping\n", task.Name, err)
				continue
			}

			// 创建自定义同步任务函数
			taskFunc = createCustomSyncTaskFunc(task.Name, task.CustomParams)
		} else {
			fmt.Printf("Unknown task type: %s for task %s\n", task.TaskType, task.Name)
			continue
		}

		entryID, err := TaskCron.AddFunc(task.Schedule, taskFunc)
		if err != nil {
			fmt.Printf("Error adding task %s: %v\n", task.Name, err)
			continue
		}

		// 保存到各种映射中
		taskIDs[task.Name] = entryID
		taskSchedules[task.Name] = task.Schedule
		taskFunctions[task.Name] = task.Function

		// 保存完整的任务信息
		taskCopy := task // 创建副本
		taskInfos[task.Name] = &taskCopy

		loadedCount++
	}
	fmt.Printf("Tasks loaded from file: %d/%d successful\n", loadedCount, len(tasks))
}

func TaskCronHandler(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "task.html", gin.H{
		"title": "任务调度器",
	})
}

// taskPageAuth 任务页面专用认证中间件
func taskPageAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从多个来源获取API密钥
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("apiKey")
		}
		if apiKey == "" {
			apiKey = c.Query("apikey") // 尝试小写版本
		}

		logging.Info("Task页面认证检查",
			"apiKey", apiKey,
			"expected", config.MediaServer.AUTH,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"header_X-API-Key", c.GetHeader("X-API-Key"),
			"query_apiKey", c.Query("apiKey"),
			"query_apikey", c.Query("apikey"))

		// 验证API密钥
		if apiKey == "" || apiKey != config.MediaServer.AUTH {
			logging.Warning("Task页面认证失败",
				"apiKey", apiKey,
				"expected", config.MediaServer.AUTH,
				"reason", func() string {
					if apiKey == "" {
						return "API Key为空"
					}
					return "API Key不匹配"
				}())
			// 重定向到登录页面，添加来源标识
			c.Redirect(http.StatusFound, "/login?from=task")
			c.Abort()
			return
		}

		logging.Info("Task页面认证成功", "apiKey", apiKey)
		c.Next()
	}
}

func TaskCronRouter(router *gin.Engine) {
	// 添加调试路由
	router.GET("/task/debug", func(c *gin.Context) {
		logging.Info("Task调试路由被访问", "path", c.Request.URL.Path, "query", c.Request.URL.RawQuery)
		c.JSON(200, gin.H{
			"message": "Task debug route works",
			"path":    c.Request.URL.Path,
			"query":   c.Request.URL.RawQuery,
		})
	})

	// 任务CRUD操作
	router.POST("/task", apiKeyAuth(), addTask)            // 创建任务
	router.GET("/tasks", apiKeyAuth(), listTasks)          // 获取任务列表
	router.GET("/task/:name", apiKeyAuth(), getTaskDetail) // 获取单个任务详情
	router.PUT("/task/:name", apiKeyAuth(), updateTask)    // 更新任务
	router.DELETE("/task/:name", apiKeyAuth(), deleteTask) // 删除任务

	// 专门的自定义同步任务端点
	router.POST("/task/custom-sync", apiKeyAuth(), addCustomSyncTask) // 创建自定义同步任务

	// 其他端点
	router.GET("/task/functions", apiKeyAuth(), getTaskFunctions)          // 获取可用函数列表
	router.GET("/task/manager/status", apiKeyAuth(), getTaskManagerStatus) // 获取任务管理器状态
	router.GET("/task", taskPageAuth(), TaskCronHandler)                   // 任务管理页面
}
