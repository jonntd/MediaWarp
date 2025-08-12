package process

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// ProcessManager 进程管理器
type ProcessManager struct {
	processes map[string]*exec.Cmd
	mutex     sync.RWMutex
}

// NewProcessManager 创建新的进程管理器
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*exec.Cmd),
	}
}

// RunWithTimeout 带超时的命令执行
func (pm *ProcessManager) RunWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) error {
	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// 创建命令
	cmd := exec.CommandContext(timeoutCtx, name, args...)
	
	// 生成进程ID
	processID := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
	
	// 注册进程
	pm.mutex.Lock()
	pm.processes[processID] = cmd
	pm.mutex.Unlock()
	
	// 执行完成后清理
	defer func() {
		pm.mutex.Lock()
		delete(pm.processes, processID)
		pm.mutex.Unlock()
	}()
	
	return cmd.Run()
}

// RunWithOutput 带输出的命令执行
func (pm *ProcessManager) RunWithOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	cmd := exec.CommandContext(timeoutCtx, name, args...)
	
	processID := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
	
	pm.mutex.Lock()
	pm.processes[processID] = cmd
	pm.mutex.Unlock()
	
	defer func() {
		pm.mutex.Lock()
		delete(pm.processes, processID)
		pm.mutex.Unlock()
	}()
	
	return cmd.Output()
}

// KillAll 终止所有进程
func (pm *ProcessManager) KillAll() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	for id, cmd := range pm.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(pm.processes, id)
	}
}

// GetRunningCount 获取运行中的进程数量
func (pm *ProcessManager) GetRunningCount() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	return len(pm.processes)
}

// 全局进程管理器
var GlobalProcessManager = NewProcessManager()

// 便捷函数
func RunWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) error {
	return GlobalProcessManager.RunWithTimeout(ctx, timeout, name, args...)
}

func RunWithOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return GlobalProcessManager.RunWithOutput(ctx, timeout, name, args...)
}
