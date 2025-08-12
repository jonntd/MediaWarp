package security

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

// 安全的命令参数验证
var (
	// 允许的字符：字母、数字、路径分隔符、常见符号
	safeArgPattern = regexp.MustCompile(`^[a-zA-Z0-9\-_./:\\]+$`)
	// 危险字符检测
	dangerousChars = []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "[", "]", "<", ">", "\"", "'"}
)

// ValidateCommandArg 验证命令参数安全性
func ValidateCommandArg(arg string) error {
	if arg == "" {
		return errors.New("参数不能为空")
	}

	// 检查危险字符
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return errors.New("参数包含危险字符: " + char)
		}
	}

	// 检查路径遍历
	if strings.Contains(arg, "..") || strings.Contains(arg, "~") {
		return errors.New("参数包含路径遍历字符")
	}

	return nil
}

// SafeExecCommand 安全的命令执行
func SafeExecCommand(name string, args ...string) (*exec.Cmd, error) {
	// 验证命令名
	if err := ValidateCommandArg(name); err != nil {
		return nil, errors.New("不安全的命令名: " + err.Error())
	}

	// 验证所有参数
	for i, arg := range args {
		if err := ValidateCommandArg(arg); err != nil {
			return nil, errors.New("不安全的参数[" + string(rune(i)) + "]: " + err.Error())
		}
	}

	return exec.Command(name, args...), nil
}

// ValidatePath 验证路径安全性
func ValidatePath(path string) error {
	// 空路径表示根目录，是允许的
	if path == "" {
		return nil
	}

	// 检查路径遍历
	if strings.Contains(path, "..") {
		return errors.New("路径包含上级目录引用")
	}

	// 检查绝对路径（根据需求调整）
	// 只有真正的系统绝对路径才被拒绝，允许以 / 开头的相对路径（如 /video/...）
	if strings.HasPrefix(path, "/") &&
		(strings.HasPrefix(path, "/etc/") ||
			strings.HasPrefix(path, "/usr/") ||
			strings.HasPrefix(path, "/var/") ||
			strings.HasPrefix(path, "/root/") ||
			strings.HasPrefix(path, "/home/") ||
			strings.HasPrefix(path, "/bin/") ||
			strings.HasPrefix(path, "/sbin/") ||
			strings.HasPrefix(path, "/sys/") ||
			strings.HasPrefix(path, "/proc/") ||
			strings.HasPrefix(path, "/dev/")) {
		return errors.New("不允许访问系统路径")
	}

	// 检查危险字符
	for _, char := range []string{";", "|", "&", "$", "`"} {
		if strings.Contains(path, char) {
			return errors.New("路径包含危险字符: " + char)
		}
	}

	return nil
}

// SanitizeServerAddr 清理服务器地址
func SanitizeServerAddr(addr string) (string, error) {
	if addr == "" {
		return "", errors.New("服务器地址不能为空")
	}

	// 只允许字母数字和常见符号
	if !safeArgPattern.MatchString(addr) {
		return "", errors.New("服务器地址格式不正确")
	}

	return addr, nil
}
