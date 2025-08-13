package config

import "MediaWarp/constants"

// 程序版本信息
type VersionInfo struct {
	AppVersion string // 程序版本号
	CommitHash string // GIt Commit Hash
	BuildData  string // 编译时间
	GoVersion  string // 编译 Golang 版本
	OS         string // 操作系统
	Arch       string //  架构
}

// 上游媒体服务器相关设置
type MediaServerSetting struct {
	Type   constants.MediaServerType // 媒体服务器类型
	ADDR   string                    // 地址
	AUTH   string                    // 认证授权KEY
	Cookie string                    // Cookie
}

// 日志设置
type LoggerSetting struct {
	AccessLogger  BaseLoggerSetting // 访问日志相关配置
	ServiceLogger BaseLoggerSetting // 服务日志相关配置
}

// 基础日志配置字段
type BaseLoggerSetting struct {
	Console bool // 是否将日志输出到终端中
	File    bool // 是否将日志输出到文件中
}

// Web前端自定义设置
type WebSetting struct {
	Enable            bool   // 启用自定义前端设置
	Custom            bool   // 启用用户自定义静态资源
	Index             bool   // 是否从 custom 目录读取 index.html 文件作为首页
	Head              string // 添加到 index.html 的 HEAD 中
	ExternalPlayerUrl bool   // 是否开启外置播放器
	Crx               bool   // crx 美化
	ActorPlus         bool   // 过滤没有头像的演员和制作人员
	FanartShow        bool   // 显示同人图（fanart图）
	Danmaku           bool   // Web 弹幕
	VideoTogether     bool   // VideoTogether
}

// 客户端User-Agent过滤设置
type ClientFilterSetting struct {
	Enable     bool
	Mode       constants.FliterMode
	ClientList []string
}

// MediaSync 媒体服务器配置（合并 HTTPStrm 和 RcloneSync）
type MediaSyncSetting []MediaSyncServerSetting

// MediaSyncServerSetting 媒体同步服务器设置
type MediaSyncServerSetting struct {
	Name      string `yaml:"Name" json:"name"`
	Remote    string `yaml:"Remote" json:"remote"`
	LocalPath string `yaml:"LocalPath" json:"local_path"`
}

// 字幕设置
type SubtitleSetting struct {
	Enable   bool
	SRT2ASS  bool // SRT 字幕转 ASS 字幕
	ASSStyle []string
	SubSet   bool // ASS 字幕字体子集化
}

// Config 统一配置结构体（用于验证）
type Config struct {
	Server          *ServerConfig      `yaml:"server" json:"server"`
	MediaServer     *MediaServerConfig `yaml:"media_server" json:"media_server"`
	HTTPStrmConfigs []*HTTPStrmConfig  `yaml:"http_strm_configs" json:"http_strm_configs"`
	Log             *LogConfig         `yaml:"log" json:"log"`
	Features        *FeaturesConfig    `yaml:"features" json:"features"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// MediaServerConfig 媒体服务器配置
type MediaServerConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
	AUTH string `yaml:"auth" json:"auth"`
	Type string `yaml:"type" json:"type"`
}

// AlistStrmConfig has been removed

// HTTPStrmConfig HTTP Strm配置
type HTTPStrmConfig struct {
	PrefixPath string `yaml:"prefix_path" json:"prefix_path"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level" json:"level"`
	Output string `yaml:"output" json:"output"`
	File   string `yaml:"file" json:"file"`
}

// FeaturesConfig 功能配置
type FeaturesConfig struct {
	EnableCache   bool `yaml:"enable_cache" json:"enable_cache"`
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics"`
}
