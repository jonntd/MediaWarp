package controllers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskManager struct {
	mu      sync.Mutex
	running bool
	cond    *sync.Cond
}

// 创建 TaskManager 的实例
func NewTaskManager() *TaskManager {
	tm := &TaskManager{}
	tm.cond = sync.NewCond(&tm.mu)
	return tm
}

// 任务执行函数，确保同一时间只有一个任务在运行
func (tm *TaskManager) RunTask(handler func()) {
	go func() { // 任务执行在新的 Goroutine 中完全异步化
		tm.mu.Lock()
		for tm.running { // 如果有任务在运行，等待
			tm.cond.Wait()
		}
		tm.running = true
		tm.mu.Unlock()

		handler() // 执行任务

		time.Sleep(30 * time.Second) // 任务结束后等待 30 秒

		tm.mu.Lock()
		tm.running = false
		tm.cond.Signal() // 通知下一个任务可以开始执行
		tm.mu.Unlock()
	}()
}

var taskManager = NewTaskManager() // 定义全局任务管理器，只允许一个任务同时运行

func MediaFileSyncHandler(ctx *gin.Context) {
	fullPath := ctx.Param("path")
	logger.ServerLogger.Info(fullPath)
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

	sourceDir := config.Remote + ":" + fullPath
	taskManager.RunTask(func() {
		syncAndCreateEmptyFiles(sourceDir, config.MountPath)
	})

}
func syncAndCreateEmptyFiles(sourceDir, remoteDest string) {
	colonIndex := strings.Index(sourceDir, ":")

	// 使用 sync 命令进行同步
	err := runRcloneSync(sourceDir, remoteDest, colonIndex)
	if err != nil {
		fmt.Printf("Error during sync: %v\n", err)
		return
	}

	// 使用 lsf 命令列出文件并创建 .strm 文件
	err = createStrmFiles(sourceDir, remoteDest, colonIndex)
	if err != nil {
		fmt.Printf("Error creating .strm files: %v\n", err)
		return
	}
	scanMediaLibrary()
}

func runRcloneSync(sourceDir, remoteDest string, colonIndex int) error {
	cmd := exec.Command("rclone", "sync", sourceDir, filepath.Join(remoteDest, sourceDir[colonIndex+1:]), "--tpslimit", "0.5", "--fast-list", "--checkers", "2", "--log-level", "INFO", "--delete-after", "--size-only", "--ignore-times", "--ignore-existing", "--ignore-checksum", "--max-size", "10M", "--transfers", "2", "--multi-thread-streams", "0", "--local-encoding", "Slash,InvalidUtf8", "--115-encoding", "Slash,InvalidUtf8", "--exclude", "*.strm")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating StderrPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// 读取 stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println("stdout:", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading stdout:", err)
		}
	}()

	// 读取 stderr 并删除目录
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("stderr:", line)

			re := regexp.MustCompile(`INFO\s+: (.+?): Removing directory`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				folderPath := filepath.Join(remoteDest, sourceDir[colonIndex+1:], matches[1])
				if err := os.RemoveAll(folderPath); err != nil {
					fmt.Printf("Failed to delete folder: %v\n", err)
				} else {
					fmt.Printf("Folder successfully deleted: %s\n", folderPath)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading stderr:", err)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error waiting for command: %v", err)
	}

	return nil
}

func createStrmFiles(sourceDir, remoteDest string, colonIndex int) error {
	cmd := exec.Command("rclone", "lsf", "-R", sourceDir, "-vv", "--files-only", "--min-size", "100M", "--checkers", "2", "--transfers", "2", "--multi-thread-streams", "0", "--local-encoding", "Slash,InvalidUtf8", "--115-encoding", "Slash,InvalidUtf8", "--tpslimit", "0.5")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		filePath := scanner.Text()
		fileName := filepath.Base(filePath)
		relativePath := filepath.Dir(filePath)

		// 构造目标路径
		destinationPath := filepath.Join(remoteDest, sourceDir[colonIndex+1:], relativePath)
		if err := os.MkdirAll(destinationPath, os.ModePerm); err != nil {
			fmt.Printf("Error creating directories: %v\n", err)
			continue
		}

		outFilePath := filepath.Join(destinationPath, fileName)
		strmFilePath := strings.TrimSuffix(outFilePath, filepath.Ext(outFilePath)) + ".strm"
		if _, err := os.Stat(strmFilePath); os.IsNotExist(err) {
			// 创建 .strm 文件
			file, err := os.Create(strmFilePath)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
				continue
			}
			defer file.Close()

			_, err = file.WriteString(outFilePath + "\n")
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
			} else {
				fmt.Printf("Empty file created: %s\n", strmFilePath)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error waiting for command: %v", err)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading command output: %v", err)
	}

	return nil
}

type Task struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

func scanMediaLibrary() {
	// 设置 Emby 服务器信息
	embyServer := config.Origin
	apiKey := config.ApiKey // 替换为实际的 API 密钥
	// 构建获取任务列表的 URL
	url := fmt.Sprintf("%s/emby/ScheduledTasks?api_key=%s", embyServer, apiKey)

	// 发送 GET 请求获取任务列表
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := ioutil.ReadAll(resp.Body)
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
		if apiKey != config.ApiKey {
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
	if apiKey == config.ApiKey {
		c.JSON(http.StatusOK, gin.H{"message": "API Key is valid"})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API Key"})
	}
}

// IndexHandler renders the main page with the folder structure
func SyncfolderHandler(ctx *gin.Context) {
	apiKey := ctx.Query("apikey")
	path := ctx.Query("path")
	if path == "" {
		path = ""
	}
	var folders []string
	var err error
	if apiKey == config.ApiKey {
		// Run the rclone lsf command to get the directory listing
		cmd := exec.Command("rclone", "lsf", "115:"+path, "--dirs-only")
		output, err := cmd.Output()
		if err != nil {
			ctx.String(http.StatusInternalServerError, fmt.Sprintf("Error running rclone: %v", err))
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
	}
	// Render the template
	tmpl := template.Must(template.ParseFiles("static/syncFolder.html"))
	err = tmpl.Execute(ctx.Writer, struct {
		Path    string
		Folders []string
	}{
		Path:    path,
		Folders: folders,
	})
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("Error rendering template: %v", err))
		return
	}
}

// Handle sync folder requests
func handleSyncFolder(ctx *gin.Context) {
	path := ctx.Param("path")
	fmt.Printf("Sync requested for folder: %s\n", path)
	ctx.JSON(http.StatusOK, gin.H{"message": "Sync request received", "path": path})
}

func SyncFilesRouter(router *gin.Engine) {
	// API to verify API Key
	router.POST("/verify", verifyAPIKey)
	// Routes with API Key auth
	router.GET("/syncfolder", SyncfolderHandler)
	router.POST("/Sync/*path", apiKeyAuth(), MediaFileSyncHandler)
	// router.POST("/Sync/*path", apiKeyAuth(), handleSyncFolder)

}
