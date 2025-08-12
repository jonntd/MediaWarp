package errors

import (
	"MediaWarp/internal/logging"
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

// ErrorCode 错误代码类型
type ErrorCode string

const (
	// 通用错误
	ErrInternalServer ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrBadRequest     ErrorCode = "BAD_REQUEST"
	ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrNotFound       ErrorCode = "NOT_FOUND"
	ErrTimeout        ErrorCode = "TIMEOUT"
	
	// 业务错误
	ErrInvalidPath      ErrorCode = "INVALID_PATH"
	ErrCommandFailed    ErrorCode = "COMMAND_FAILED"
	ErrCacheError       ErrorCode = "CACHE_ERROR"
	ErrConfigError      ErrorCode = "CONFIG_ERROR"
	ErrNetworkError     ErrorCode = "NETWORK_ERROR"
	ErrAuthError        ErrorCode = "AUTH_ERROR"
	ErrSyncError        ErrorCode = "SYNC_ERROR"
)

// AppError 应用错误结构
type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	StatusCode int       `json:"-"`
	Cause      error     `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAppError 创建新的应用错误
func NewAppError(code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// NewAppErrorWithCause 创建带原因的应用错误
func NewAppErrorWithCause(code ErrorCode, message string, statusCode int, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// 预定义错误
var (
	ErrInvalidPathParam = NewAppError(ErrInvalidPath, "Invalid path parameter", http.StatusBadRequest)
	ErrInvalidServerAddr = NewAppError(ErrBadRequest, "Invalid server address", http.StatusBadRequest)
	ErrCommandExecution = NewAppError(ErrCommandFailed, "Command execution failed", http.StatusInternalServerError)
	ErrCacheOperation   = NewAppError(ErrCacheError, "Cache operation failed", http.StatusInternalServerError)
	ErrNetworkRequest   = NewAppError(ErrNetworkError, "Network request failed", http.StatusBadGateway)
	ErrAuthFailed       = NewAppError(ErrAuthError, "Authentication failed", http.StatusUnauthorized)
	ErrSyncOperation    = NewAppError(ErrSyncError, "Sync operation failed", http.StatusInternalServerError)
)

// ErrorResponse HTTP错误响应结构
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   ErrorCode `json:"error"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	TraceID string    `json:"trace_id,omitempty"`
}

// HandleError 统一错误处理中间件
func HandleError() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		
		// 检查是否有错误
		if len(ctx.Errors) > 0 {
			err := ctx.Errors.Last()
			
			// 记录错误堆栈
			pc, file, line, _ := runtime.Caller(1)
			funcName := runtime.FuncForPC(pc).Name()
			
			logging.Error("Error occurred:",
				"error", err.Error(),
				"file", file,
				"line", line,
				"func", funcName,
				"path", ctx.Request.URL.Path,
				"method", ctx.Request.Method,
			)
			
			// 处理不同类型的错误
			if appErr, ok := err.Err.(*AppError); ok {
				HandleAppError(ctx, appErr)
			} else {
				// 未知错误，返回通用错误
				HandleAppError(ctx, NewAppErrorWithCause(
					ErrInternalServer,
					"An unexpected error occurred",
					http.StatusInternalServerError,
					err.Err,
				))
			}
		}
	}
}

// HandleAppError 处理应用错误
func HandleAppError(ctx *gin.Context, err *AppError) {
	// 生成追踪ID
	traceID := generateTraceID()
	
	// 记录详细错误信息
	logging.Error("Application error:",
		"code", err.Code,
		"message", err.Message,
		"status", err.StatusCode,
		"trace_id", traceID,
		"cause", err.Cause,
	)
	
	// 构建响应
	response := ErrorResponse{
		Success: false,
		Error:   err.Code,
		Message: err.Message,
		TraceID: traceID,
	}
	
	// 在开发环境下添加详细信息
	if gin.Mode() == gin.DebugMode && err.Cause != nil {
		response.Details = err.Cause.Error()
	}
	
	ctx.JSON(err.StatusCode, response)
	ctx.Abort()
}

// AbortWithError 中止请求并返回错误
func AbortWithError(ctx *gin.Context, err *AppError) {
	ctx.Error(err)
	ctx.Abort()
}

// generateTraceID 生成追踪ID
func generateTraceID() string {
	return fmt.Sprintf("trace_%d", runtime.NumGoroutine())
}

// RecoverMiddleware 恢复中间件，处理panic
func RecoverMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录panic信息
				stack := make([]byte, 4096)
				length := runtime.Stack(stack, false)
				
				logging.Error("Panic recovered:",
					"error", err,
					"stack", string(stack[:length]),
					"path", ctx.Request.URL.Path,
					"method", ctx.Request.Method,
				)
				
				// 返回内部服务器错误
				HandleAppError(ctx, NewAppError(
					ErrInternalServer,
					"Internal server error",
					http.StatusInternalServerError,
				))
			}
		}()
		
		ctx.Next()
	}
}

// 便捷函数
func BadRequest(message string) *AppError {
	return NewAppError(ErrBadRequest, message, http.StatusBadRequest)
}

func InternalError(message string, cause error) *AppError {
	return NewAppErrorWithCause(ErrInternalServer, message, http.StatusInternalServerError, cause)
}

func NotFound(message string) *AppError {
	return NewAppError(ErrNotFound, message, http.StatusNotFound)
}

func Unauthorized(message string) *AppError {
	return NewAppError(ErrUnauthorized, message, http.StatusUnauthorized)
}
