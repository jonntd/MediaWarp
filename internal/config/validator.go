package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("配置验证失败 [%s]: %s (当前值: %v)", e.Field, e.Message, e.Value)
}

// ConfigValidator 配置验证器
type ConfigValidator struct {
	errors []ValidationError
}

// NewConfigValidator 创建新的配置验证器
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		errors: make([]ValidationError, 0),
	}
}

// AddError 添加验证错误
func (cv *ConfigValidator) AddError(field string, value interface{}, message string) {
	cv.errors = append(cv.errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// HasErrors 检查是否有错误
func (cv *ConfigValidator) HasErrors() bool {
	return len(cv.errors) > 0
}

// GetErrors 获取所有错误
func (cv *ConfigValidator) GetErrors() []ValidationError {
	return cv.errors
}

// GetErrorsAsString 获取错误字符串
func (cv *ConfigValidator) GetErrorsAsString() string {
	if !cv.HasErrors() {
		return ""
	}
	
	var messages []string
	for _, err := range cv.errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

// ValidateRequired 验证必填字段
func (cv *ConfigValidator) ValidateRequired(field string, value interface{}) {
	if value == nil {
		cv.AddError(field, value, "字段不能为空")
		return
	}
	
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			cv.AddError(field, value, "字符串不能为空")
		}
	case int, int32, int64:
		// 数字类型默认不为空
	case []interface{}:
		if len(v) == 0 {
			cv.AddError(field, value, "数组不能为空")
		}
	}
}

// ValidateURL 验证URL格式
func (cv *ConfigValidator) ValidateURL(field string, value string) {
	if value == "" {
		return // 空值由 ValidateRequired 处理
	}
	
	if _, err := url.Parse(value); err != nil {
		cv.AddError(field, value, "URL格式不正确: "+err.Error())
	}
}

// ValidatePort 验证端口号
func (cv *ConfigValidator) ValidatePort(field string, value interface{}) {
	var port int
	
	switch v := value.(type) {
	case int:
		port = v
	case string:
		var err error
		port, err = strconv.Atoi(v)
		if err != nil {
			cv.AddError(field, value, "端口号必须是数字")
			return
		}
	default:
		cv.AddError(field, value, "端口号类型不正确")
		return
	}
	
	if port < 1 || port > 65535 {
		cv.AddError(field, value, "端口号必须在1-65535之间")
	}
}

// ValidatePath 验证路径
func (cv *ConfigValidator) ValidatePath(field string, value string, mustExist bool) {
	if value == "" {
		return // 空值由 ValidateRequired 处理
	}
	
	// 检查路径格式
	if !filepath.IsAbs(value) && !strings.HasPrefix(value, "./") && !strings.HasPrefix(value, "../") {
		cv.AddError(field, value, "路径必须是绝对路径或相对路径")
		return
	}
	
	// 检查路径是否存在
	if mustExist {
		if _, err := os.Stat(value); os.IsNotExist(err) {
			cv.AddError(field, value, "路径不存在")
		}
	}
}

// ValidateRegex 验证正则表达式
func (cv *ConfigValidator) ValidateRegex(field string, value string) {
	if value == "" {
		return // 空值由 ValidateRequired 处理
	}
	
	if _, err := regexp.Compile(value); err != nil {
		cv.AddError(field, value, "正则表达式格式不正确: "+err.Error())
	}
}

// ValidateEnum 验证枚举值
func (cv *ConfigValidator) ValidateEnum(field string, value string, validValues []string) {
	if value == "" {
		return // 空值由 ValidateRequired 处理
	}
	
	for _, valid := range validValues {
		if value == valid {
			return
		}
	}
	
	cv.AddError(field, value, fmt.Sprintf("值必须是以下之一: %s", strings.Join(validValues, ", ")))
}

// ValidateRange 验证数值范围
func (cv *ConfigValidator) ValidateRange(field string, value, min, max int) {
	if value < min || value > max {
		cv.AddError(field, value, fmt.Sprintf("值必须在%d-%d之间", min, max))
	}
}

// ValidateConfig 验证完整配置
func ValidateConfig(cfg *Config) error {
	validator := NewConfigValidator()
	
	// 验证媒体服务器配置
	if cfg.MediaServer != nil {
		validator.ValidateRequired("MediaServer.Host", cfg.MediaServer.Host)
		validator.ValidatePort("MediaServer.Port", cfg.MediaServer.Port)
		validator.ValidateURL("MediaServer.Host", fmt.Sprintf("http://%s:%d", cfg.MediaServer.Host, cfg.MediaServer.Port))
		validator.ValidateRequired("MediaServer.AUTH", cfg.MediaServer.AUTH)
		
		// 验证服务器类型
		if cfg.MediaServer.Type != "" {
			validator.ValidateEnum("MediaServer.Type", cfg.MediaServer.Type, []string{"emby", "jellyfin", "plex"})
		}
	}
	
	// 验证Alist配置
	if cfg.AlistStrmConfigs != nil {
		for i, alistConfig := range cfg.AlistStrmConfigs {
			fieldPrefix := fmt.Sprintf("AlistStrmConfigs[%d]", i)
			
			validator.ValidateRequired(fieldPrefix+".ADDR", alistConfig.ADDR)
			validator.ValidateRequired(fieldPrefix+".Username", alistConfig.Username)
			validator.ValidateRequired(fieldPrefix+".Password", alistConfig.Password)
			validator.ValidateURL(fieldPrefix+".Host", alistConfig.Host)
			
			if alistConfig.Type != "" {
				validator.ValidateEnum(fieldPrefix+".Type", alistConfig.Type, []string{"alist", "webdav"})
			}
		}
	}
	
	// 验证HTTP Strm配置
	if cfg.HTTPStrmConfigs != nil {
		for i, httpConfig := range cfg.HTTPStrmConfigs {
			fieldPrefix := fmt.Sprintf("HTTPStrmConfigs[%d]", i)
			
			validator.ValidateRequired(fieldPrefix+".PrefixPath", httpConfig.PrefixPath)
			validator.ValidatePath(fieldPrefix+".PrefixPath", httpConfig.PrefixPath, false)
		}
	}
	
	// 验证服务器配置
	if cfg.Server != nil {
		validator.ValidatePort("Server.Port", cfg.Server.Port)
		validator.ValidateRange("Server.Port", cfg.Server.Port, 1024, 65535)
		
		if cfg.Server.Host != "" {
			validator.ValidateURL("Server.Host", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port))
		}
	}
	
	// 验证日志配置
	if cfg.Log != nil {
		if cfg.Log.Level != "" {
			validator.ValidateEnum("Log.Level", cfg.Log.Level, []string{"debug", "info", "warn", "error", "fatal"})
		}
		
		if cfg.Log.Output != "" {
			validator.ValidateEnum("Log.Output", cfg.Log.Output, []string{"stdout", "file", "both"})
		}
		
		if cfg.Log.File != "" {
			// 验证日志文件目录是否存在
			dir := filepath.Dir(cfg.Log.File)
			validator.ValidatePath("Log.File", dir, true)
		}
	}
	
	// 验证功能开关
	if cfg.Features != nil {
		// 这里可以添加功能相关的验证
	}
	
	if validator.HasErrors() {
		return errors.New(validator.GetErrorsAsString())
	}
	
	return nil
}

// ValidateEnvironment 验证环境变量
func ValidateEnvironment() error {
	validator := NewConfigValidator()
	
	// 检查必要的环境变量
	requiredEnvs := []string{
		// 可以根据需要添加必需的环境变量
	}
	
	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			validator.AddError("Environment", env, "环境变量未设置")
		}
	}
	
	if validator.HasErrors() {
		return errors.New(validator.GetErrorsAsString())
	}
	
	return nil
}

// SanitizeConfig 清理和标准化配置
func SanitizeConfig(cfg *Config) {
	// 清理字符串字段的空白字符
	if cfg.MediaServer != nil {
		cfg.MediaServer.Host = strings.TrimSpace(cfg.MediaServer.Host)
		cfg.MediaServer.AUTH = strings.TrimSpace(cfg.MediaServer.AUTH)
		cfg.MediaServer.Type = strings.ToLower(strings.TrimSpace(cfg.MediaServer.Type))
	}
	
	// 标准化路径
	if cfg.HTTPStrmConfigs != nil {
		for i := range cfg.HTTPStrmConfigs {
			cfg.HTTPStrmConfigs[i].PrefixPath = filepath.Clean(cfg.HTTPStrmConfigs[i].PrefixPath)
		}
	}
	
	// 设置默认值
	if cfg.Server != nil && cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	
	if cfg.Log != nil {
		if cfg.Log.Level == "" {
			cfg.Log.Level = "info"
		}
		if cfg.Log.Output == "" {
			cfg.Log.Output = "stdout"
		}
	}
}
