package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/viper"
)

func make_config() {
	remoteName := config.Remote
	remoteType := config.Remote
	cookie := config.Cookie
	// pacerMinSleep := config.PacerMinSleep
	cmdDelete := exec.Command("rclone", "config", "delete", remoteName)
	if output, err := cmdDelete.CombinedOutput(); err != nil {
		fmt.Printf("Failed to delete existing config: %s\nOutput: %s", err, string(output))
	}

	cmd := exec.Command("rclone", "config", "create", remoteName, remoteType,
		"cookie", cookie,
		// "pacer_min_sleep", pacerMinSleep,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to execute command: %s\nOutput: %s", err, string(output))
	}
	fmt.Printf("Command output: %s", string(output))
}

type serverConfig struct {
	Host string
	Port int
}

type loggerConfig struct {
	Enable        bool
	AccessLogger  baseLoggerConfig
	ServiceLogger baseLoggerConfig
}

type baseLoggerConfig struct {
	Enable bool
}

type configManager struct {
	Server        serverConfig
	LoggerSetting loggerConfig
	Origin        string
	ApiKey        string
	Cookie        string
	Remote        string
	MountPath     string
	Debug         bool
	PacerMinSleep string
}

// 读取并解析配置文件
func (c *configManager) LoadConfig() {
	vip := viper.New()
	vip.SetConfigFile(c.ConfigPath())
	vip.SetConfigType("yaml")
	// vip.SetConfigName("config")
	// vip.AddConfigPath(".")
	if err := vip.ReadInConfig(); err != nil {
		panic(err)
	}

	err := vip.Unmarshal(c)
	if err != nil {
		panic(err)
	}
}

// 创建文件夹
func (c *configManager) CreateDir() {
	if err := os.MkdirAll(c.ConfigDir(), os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(c.LogDir(), os.ModePerm); err != nil {
		panic(err)
	}
}

// 初始化configManager
func (c *configManager) Init() {
	c.LoadConfig()
	c.CreateDir()
	make_config()

}

// 获取版本号
func (c *configManager) Version() string {
	return APP_VERSION
}

// 获取项目根目录
func (c *configManager) RootDir() string {
	// _, fullFilename, _, _ := runtime.Caller(0)
	// return filepath.Dir(filepath.Dir(fullFilename))

	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get working directory: %v", err)
	}
	return dir

}

// 获取配置文件目录
func (c *configManager) ConfigDir() string {
	return filepath.Join(c.RootDir(), "config")
}

// 获取配置文件路径
func (c *configManager) ConfigPath() string {
	return filepath.Join(c.ConfigDir(), "config.yaml")
}

// 获取日志目录
func (c *configManager) LogDir() string {
	return filepath.Join(c.RootDir(), "logs")
}

// 获取访问日志文件路径
func (c *configManager) AccessLogPath() string {
	return filepath.Join(c.LogDir(), "access.log")
}

// 获取服务日志文件路径
func (c *configManager) ServiceLogPath() string {
	return filepath.Join(c.LogDir(), "service.log")
}

// MediaWarp监听地址
func (c *configManager) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// -----------------外部引用部分----------------- //
var config configManager

func GetConfig() *configManager {
	return &config
}

func init() {
	config.Init()
}
