package main

import (
	"MediaWarp/constants"
	"MediaWarp/internal/config"
	"MediaWarp/internal/handler"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/router"
	"MediaWarp/internal/service"
	"MediaWarp/utils"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	isDebug     bool   // 开启调试模式
	showVersion bool   // 显示版本信息
	configPath  string // 配置文件路径
)

func init() {
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&isDebug, "debug", false, "是否启用调试模式")
	flag.StringVar(&configPath, "config", "", "指定配置文件路径")
	flag.Parse()

	fmt.Print(constants.LOGO)
	fmt.Println(utils.Center(fmt.Sprintf(" MediaWarp %s ", config.Version().AppVersion), 71, "="))
}

func make_config() {
	logging.Info("make rclone config file")
	var configNotFoundMsg = "not found - using defaults"
	cmd := exec.Command("rclone", "listremotes")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	logging.Info("rclone config: " + outputStr)
	if strings.Contains(outputStr, configNotFoundMsg) {
		for _, alistStrmConfig := range config.AlistStrm.List {
			logging.Info("alistStrmConfig.ADDR: " + alistStrmConfig.ADDR)
			logging.Info("alistStrmConfig.Cookie: " + alistStrmConfig.Cookie)
			logging.Info("alistStrmConfig.Type: " + alistStrmConfig.Type)
			cmd := exec.Command("rclone", "config", "create", alistStrmConfig.ADDR, alistStrmConfig.Type, "cookie", alistStrmConfig.Cookie)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logging.Debug(fmt.Sprintf("Failed to execute command: %s\nOutput: %s", err, string(output)))
			}
			logging.Debug(fmt.Sprintf("Command output: %s", string(output)))
			// rclone config reconnect 115:
			cmd = exec.Command("rclone", "lsf", string(alistStrmConfig.ADDR)+":")
			output, err = cmd.CombinedOutput()
			if err != nil {
				logging.Debug(fmt.Sprintf("Failed to execute command: %s\nOutput: %s", err, string(output)))
			}
			logging.Debug(fmt.Sprintf("Command output: %s", string(output)))
		}
	}
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			logging.Debug("rclone command not found or failed to start: %w", err)
			return
		}
	}

}

func main() {
	if showVersion {
		versionInfo, _ := json.MarshalIndent(config.Version(), "", "  ")
		fmt.Println(string(versionInfo))
		return
	}

	signChan := make(chan os.Signal, 1)
	errChan := make(chan error, 1)
	signal.Notify(signChan, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		fmt.Println("MediaWarp 已退出")
	}()

	if err := config.Init(configPath); err != nil { // 初始化配置
		fmt.Println("配置初始化失败：", err)
		return
	}
	logging.Init()                                                                           // 初始化日志
	logging.Infof("上游媒体服务器类型：%s，服务器地址：%s", config.MediaServer.Type, config.MediaServer.ADDR) // 日志打印
	service.InitAlistSerer()                                                                 // 初始化Alist服务器
	if err := handler.Init(); err != nil {                                                   // 初始化媒体服务器处理器
		logging.Error("媒体服务器处理器初始化失败：", err)
		return
	}
	if !isDebug {
		isDebug = config.Debug
	}
	if isDebug {
		logging.SetLevel(logrus.DebugLevel)
		logging.Warning("已启用调试模式")
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	// make_config()
	logging.Info("Environ ", os.Environ())
	logging.Info("MediaWarp 监听端口：", config.Port)
	ginR := router.InitRouter() // 路由初始化
	logging.Info("MediaWarp 启动成功")
	go func() {
		if err := ginR.Run(config.ListenAddr()); err != nil {
			errChan <- err
		}
	}()

	select {
	case sig := <-signChan:
		logging.Info("MediaWarp 正在退出，信号：", sig)
	case err := <-errChan:
		logging.Error("MediaWarp 运行出错：", err)
	}

}
