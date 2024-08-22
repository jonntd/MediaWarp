package controllers

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

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
	sourceDir := config.Remote + ":" + fullPath
	go syncAndCreateEmptyFiles(sourceDir, config.MountPath)

	ctx.JSON(200, gin.H{
		"message": "success",
		"path":    fullPath})
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
}

func runRcloneSync(sourceDir, remoteDest string, colonIndex int) error {
	cmd := exec.Command("rclone", "sync", sourceDir, filepath.Join(remoteDest, sourceDir[colonIndex+1:]), "--tpslimit", "5", "--update", "--fast-list", "--checkers", "4", "--log-level", "INFO", "--delete-after", "--size-only", "--ignore-times", "--ignore-existing", "--ignore-checksum", "--max-size", "10M", "--transfers", "4", "--multi-thread-streams", "2", "--local-encoding", "Slash,InvalidUtf8", "--115-encoding", "Slash,InvalidUtf8", "--exclude", "*.strm")

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
	cmd := exec.Command("rclone", "lsf", "-R", sourceDir, "-vv", "--files-only", "--min-size", "100M", "--transfers", "4", "--multi-thread-streams", "2", "--local-encoding", "Slash,InvalidUtf8", "--115-encoding", "Slash,InvalidUtf8", "--tpslimit", "5")

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