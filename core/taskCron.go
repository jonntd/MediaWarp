package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

var (
	TaskCron      *cron.Cron
	taskIDs       map[string]cron.EntryID
	taskSchedules map[string]string
	taskFunctions map[string]string // 用于存储 function 名称
	mu            sync.Mutex
)

type TaskInfo struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Function string `json:"function"`
	NextRun  string `json:"next_run,omitempty"`
}

func init() {
	TaskCron = cron.New(cron.WithSeconds())
	taskIDs = make(map[string]cron.EntryID)
	taskSchedules = make(map[string]string)
	taskFunctions = make(map[string]string) // 初始化 taskFunctions
	loadTasksFromFile()                     // 启动时加载任务
}

func customFunction1() {
	fmt.Printf("Executing custom function 1 at %s\n", time.Now())
}

func customFunction2() {
	fmt.Printf("Executing custom function 2 at %s\n", time.Now())
}

func customFunction3() {
	fmt.Printf("Executing custom function 3 at %s\n", time.Now())
}

// 定义函数映射
var taskFunctionsMap = map[string]func(){
	"func1": customFunction1,
	"func2": customFunction2,
	"func3": customFunction3,
	// 可以在这里继续添加更多函数
}

// Task function that runs once and then deletes itself
func runOnceAndDelete(taskName string, taskFunc func()) func() {
	return func() {
		// 执行任务
		taskFunc()

		// 删除任务
		mu.Lock()
		defer mu.Unlock()

		entryID, exists := taskIDs[taskName]
		if exists {
			TaskCron.Remove(entryID)
			delete(taskIDs, taskName)
			delete(taskSchedules, taskName)
			delete(taskFunctions, taskName)
			saveTasksToFile() // 删除任务后保存到文件
		}

		fmt.Printf("Task %s executed and removed\n", taskName)
	}
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

	// 查找对应的函数
	taskFunc, exists := taskFunctionsMap[taskInfo.Function]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name"})
		return
	}

	var taskToRun func()
	if isOneTimeTask(taskInfo.Schedule) {
		taskToRun = runOnceAndDelete(taskInfo.Name, taskFunc)
	} else {
		taskToRun = taskFunc
		// 添加任务后立即执行一次（可选）
		go taskToRun()
	}

	entryID, err := TaskCron.AddFunc(taskInfo.Schedule, taskToRun)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
		return
	}

	taskSchedules[taskInfo.Name] = taskInfo.Schedule
	taskIDs[taskInfo.Name] = entryID
	taskFunctions[taskInfo.Name] = taskInfo.Function // 保存 function 名称
	saveTasksToFile()                                // 添加任务后保存到文件

	c.JSON(http.StatusOK, gin.H{"status": "Task added", "task_name": taskInfo.Name})
}

func isOneTimeTask(schedule string) bool {
	// 判断是否为每年1月1日的表达式
	return schedule == "0 0 0 1 1 *"
}

func listTasks(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	tasks := make([]TaskInfo, 0, len(taskIDs))
	for name, entryID := range taskIDs {
		entry := TaskCron.Entry(entryID)
		tasks = append(tasks, TaskInfo{
			Name:     name,
			Schedule: taskSchedules[name],
			Function: taskFunctions[name], // 从 taskFunctions 读取 function 名称
			NextRun:  entry.Next.String(),
		})
	}

	c.JSON(http.StatusOK, tasks)
}

func deleteTask(c *gin.Context) {
	taskName := c.Param("name")

	mu.Lock()
	defer mu.Unlock()

	entryID, exists := taskIDs[taskName]
	if exists {
		TaskCron.Remove(entryID)
		delete(taskIDs, taskName)
		delete(taskSchedules, taskName)
		delete(taskFunctions, taskName) // 删除对应的 function 名称
		saveTasksToFile()               // 删除任务后保存到文件
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Task deleted", "task_name": taskName})
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
		tasks = append(tasks, TaskInfo{
			Name:     name,
			Schedule: taskSchedules[name],
			Function: taskFunctions[name], // 保存 function 名称
			NextRun:  entry.Next.String(),
		})
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(tasks); err != nil {
		fmt.Printf("Error encoding tasks to JSON: %v\n", err)
	}
	fmt.Printf("saveTasksToFile\n")
}

func loadTasksFromFile() {
	file, err := os.Open("tasks.json")
	if err != nil {
		if os.IsNotExist(err) {
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

	for _, task := range tasks {
		taskFunc, exists := taskFunctionsMap[task.Function]
		if !exists {
			fmt.Printf("Invalid function name: %s\n", task.Function)
			continue
		}

		var taskToRun func()
		if isOneTimeTask(task.Schedule) {
			taskToRun = runOnceAndDelete(task.Name, taskFunc)
		} else {
			taskToRun = taskFunc
		}

		entryID, err := TaskCron.AddFunc(task.Schedule, taskToRun)
		if err != nil {
			fmt.Printf("Error adding task: %v\n", err)
			continue
		}

		taskIDs[task.Name] = entryID
		taskSchedules[task.Name] = task.Schedule
		taskFunctions[task.Name] = task.Function // 加载 function 名称
	}
	fmt.Printf("loadTasksFromFile\n")
}

func SetupRouter(router *gin.Engine) {
	router.POST("/task", addTask)
	router.GET("/tasks", listTasks)
	router.DELETE("/task/:name", deleteTask)
	router.GET("/task", func(c *gin.Context) {
		c.File("./static/task.html")
	})
}
