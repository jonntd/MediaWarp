package main

import (
	"MediaWarp/constants"
	"MediaWarp/internal/cache"
	"MediaWarp/internal/config"
	"MediaWarp/internal/handler"
	"MediaWarp/internal/health"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/metrics"
	"MediaWarp/internal/process"
	"MediaWarp/internal/router"
	"MediaWarp/utils"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// Alist support has been removed
	logging.Info("Alist support has been removed, skipping rclone config creation")
}

// initializeEnhancedSystem 初始化增强系统
func initializeEnhancedSystem() error {
	logging.Info("正在初始化增强系统...")

	// 1. 验证环境
	if err := config.ValidateEnvironment(); err != nil {
		logging.Warning("环境验证警告: ", err)
		// 不中断启动，只记录警告
	}

	// 2. 初始化文件夹缓存 (15分钟TTL)
	cache.InitGlobalFolderCache(15 * time.Minute)
	logging.Info("文件夹缓存已初始化", "ttl", "15分钟")

	// 3. 添加健康检查
	health.GlobalHealthChecker.AddCheck(&health.RcloneHealthCheck{})

	// 4. 启动指标收集
	go startMetricsCollection()

	logging.Info("增强系统初始化完成")
	return nil
}

// startMetricsCollection 启动指标收集
func startMetricsCollection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 更新系统指标
		// 这里可以添加更多的指标更新逻辑
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
	// Alist service has been removed
	if err := handler.Init(); err != nil { // 初始化媒体服务器处理器
		logging.Error("媒体服务器处理器初始化失败：", err)
		return
	}

	// 初始化增强系统
	if err := initializeEnhancedSystem(); err != nil {
		logging.Error("增强系统初始化失败：", err)
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

	// 添加健康检查和监控端点
	ginR.GET("/health", health.HealthHandler)
	ginR.GET("/ready", health.ReadinessHandler)
	ginR.GET("/live", health.LivenessHandler)
	ginR.GET("/metrics", func(ctx *gin.Context) {
		allMetrics := metrics.GlobalCollector.GetAllMetrics()

		result := make(map[string]interface{})
		for name, metric := range allMetrics {
			result[name] = map[string]interface{}{
				"type":   metric.Type(),
				"value":  metric.Value(),
				"labels": metric.Labels(),
			}
		}

		ctx.JSON(200, gin.H{
			"metrics":   result,
			"timestamp": time.Now(),
		})
	})

	// 注册缓存统计API路由
	handler.RegisterCacheStatsRoutes(ginR)
	logging.Info("缓存监控API已启用")

	// Web监控系统已移除

	// 缓存预热功能已移除
	logging.Info("MediaWarp 启动成功")
	go func() {
		if err := ginR.Run(config.ListenAddr()); err != nil {
			errChan <- err
		}
	}()

	select {
	case sig := <-signChan:
		logging.Info("MediaWarp 正在退出，信号：", sig)
		gracefulShutdown()
	case err := <-errChan:
		logging.Error("MediaWarp 运行出错：", err)
		gracefulShutdown()
	}

}

// gracefulShutdown 优雅关闭
func gracefulShutdown() {
	logging.Info("正在执行优雅关闭...")

	// 停止进程管理器
	process.GlobalProcessManager.KillAll()

	// 关闭全局播放信息缓存
	if cache.GlobalPlaybackCache != nil {
		cache.GlobalPlaybackCache.Close()
		logging.Info("播放信息缓存已关闭")
	}

	logging.Info("优雅关闭完成")
}
