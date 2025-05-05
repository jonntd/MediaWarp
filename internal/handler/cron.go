package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	taskFunctions map[string]string
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
	taskFunctions = make(map[string]string)
	loadTasksFromFile()
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

var taskFunctionsMap = map[string]func(){
	"func1": customFunction1,
	"func2": customFunction2,
	"func3": customFunction3,
	// 添加更多函数
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

	taskFunc, exists := taskFunctionsMap[taskInfo.Function]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name"})
		return
	}

	go taskFunc() // 可选：立即执行一次
	entryID, err := TaskCron.AddFunc(taskInfo.Schedule, taskFunc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
		return
	}

	taskSchedules[taskInfo.Name] = taskInfo.Schedule
	taskIDs[taskInfo.Name] = entryID
	taskFunctions[taskInfo.Name] = taskInfo.Function
	saveTasksToFile()

	c.JSON(http.StatusOK, gin.H{"status": "Task added", "task_name": taskInfo.Name})
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
	fmt.Printf("taskName: %s\n", taskName)
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

	for _, task := range tasks {
		taskFunc, exists := taskFunctionsMap[task.Function]
		if !exists {
			fmt.Printf("Invalid function name: %s\n", task.Function)
			continue
		}

		entryID, err := TaskCron.AddFunc(task.Schedule, taskFunc)
		if err != nil {
			fmt.Printf("Error adding task: %v\n", err)
			continue
		}

		taskIDs[task.Name] = entryID
		taskSchedules[task.Name] = task.Schedule
		taskFunctions[task.Name] = task.Function
	}
	fmt.Printf("Tasks loaded from file\n")
}

func TaskCronHandler(ctx *gin.Context) {
	taskTemplate := template.Must(template.ParseFiles("static/task.html"))
	err := taskTemplate.Execute(ctx.Writer, nil)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "Error rendering template")
		return
	}
}

func TaskCronRouter(router *gin.Engine) {
	router.POST("/task", apiKeyAuth(), addTask)
	router.GET("/tasks", apiKeyAuth(), listTasks)
	router.DELETE("/task/:name", apiKeyAuth(), deleteTask)
	router.GET("/task", TaskCronHandler)

}
