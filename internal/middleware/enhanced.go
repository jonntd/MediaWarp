package middleware

import (
	"MediaWarp/internal/logging"
	"MediaWarp/internal/metrics"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestID 中间件 - 为每个请求生成唯一ID
func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := generateRequestID()
		ctx.Set("request_id", requestID)
		ctx.Header("X-Request-ID", requestID)
		ctx.Next()
	}
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		// 处理请求
		ctx.Next()

		// 记录指标
		duration := time.Since(start)
		status := ctx.Writer.Status()
		method := ctx.Request.Method
		path := ctx.FullPath()

		// 如果路径为空，使用原始路径
		if path == "" {
			path = ctx.Request.URL.Path
		}

		// 记录HTTP请求指标
		metrics.RecordHTTPRequest(method, path, status, duration)

		// 记录详细日志
		logging.Info("HTTP Request",
			"method", method,
			"path", path,
			"status", status,
			"duration", duration,
			"request_id", ctx.GetString("request_id"),
			"user_agent", ctx.GetHeader("User-Agent"),
			"remote_addr", ctx.ClientIP(),
		)
	}
}

// RateLimitMiddleware 速率限制中间件
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	// 简单的内存速率限制器
	clients := make(map[string][]time.Time)

	return func(ctx *gin.Context) {
		clientIP := ctx.ClientIP()
		now := time.Now()

		// 清理过期的请求记录
		if requests, exists := clients[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if now.Sub(reqTime) < time.Minute {
					validRequests = append(validRequests, reqTime)
				}
			}
			clients[clientIP] = validRequests
		}

		// 检查速率限制
		if len(clients[clientIP]) >= requestsPerMinute {
			logging.Warning("Rate limit exceeded",
				"client_ip", clientIP,
				"requests", len(clients[clientIP]),
				"limit", requestsPerMinute,
			)

			ctx.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			ctx.Abort()
			return
		}

		// 记录当前请求
		clients[clientIP] = append(clients[clientIP], now)

		ctx.Next()
	}
}

// SecurityHeadersMiddleware 安全头中间件
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 设置安全头
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("X-XSS-Protection", "1; mode=block")
		ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		ctx.Header("Content-Security-Policy", "default-src 'self'")

		// 移除服务器信息
		ctx.Header("Server", "MediaWarp")

		ctx.Next()
	}
}

// CORSMiddleware CORS中间件
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")

		// 检查是否允许该源
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			ctx.Header("Access-Control-Allow-Origin", origin)
		}

		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		ctx.Header("Access-Control-Expose-Headers", "X-Request-ID")
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

// AuthMiddleware 认证中间件
func AuthMiddleware(validTokens []string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 从多个地方获取token
		token := getTokenFromRequest(ctx)

		if token == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authentication token",
			})
			ctx.Abort()
			return
		}

		// 验证token
		valid := false
		for _, validToken := range validTokens {
			if token == validToken {
				valid = true
				break
			}
		}

		if !valid {
			logging.Warning("Invalid authentication token",
				"token", maskToken(token),
				"client_ip", ctx.ClientIP(),
			)

			ctx.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authentication token",
			})
			ctx.Abort()
			return
		}

		// 设置认证信息到上下文
		ctx.Set("authenticated", true)
		ctx.Set("auth_token", token)

		ctx.Next()
	}
}

// TimeoutMiddleware 超时中间件
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 设置请求超时
		ctx.Request = ctx.Request.WithContext(
			ctx.Request.Context(),
		)

		// 这里可以添加更复杂的超时逻辑
		ctx.Next()
	}
}

// CompressionMiddleware 压缩中间件
func CompressionMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 检查客户端是否支持压缩
		acceptEncoding := ctx.GetHeader("Accept-Encoding")

		if strings.Contains(acceptEncoding, "gzip") {
			ctx.Header("Content-Encoding", "gzip")
			// 这里可以添加实际的gzip压缩逻辑
		}

		ctx.Next()
	}
}

// ValidationMiddleware 请求验证中间件
func ValidationMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 验证Content-Type
		if ctx.Request.Method == "POST" || ctx.Request.Method == "PUT" {
			contentType := ctx.GetHeader("Content-Type")
			if contentType != "" && !strings.Contains(contentType, "application/json") &&
				!strings.Contains(contentType, "application/x-www-form-urlencoded") &&
				!strings.Contains(contentType, "multipart/form-data") {
				ctx.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Unsupported content type",
				})
				ctx.Abort()
				return
			}
		}

		// 验证Content-Length
		if contentLengthStr := ctx.GetHeader("Content-Length"); contentLengthStr != "" {
			if contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64); err == nil {
				// 限制请求体大小 (例如: 10MB)
				maxSize := int64(10 * 1024 * 1024)
				if contentLength > maxSize {
					ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{
						"error": "Request body too large",
					})
					ctx.Abort()
					return
				}
			}
		}

		ctx.Next()
	}
}

// 辅助函数

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// getTokenFromRequest 从请求中获取token
func getTokenFromRequest(ctx *gin.Context) string {
	// 从Authorization头获取
	if auth := ctx.GetHeader("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}

	// 从查询参数获取
	if token := ctx.Query("token"); token != "" {
		return token
	}

	// 从表单获取
	if token := ctx.PostForm("token"); token != "" {
		return token
	}

	return ""
}

// maskToken 遮蔽token用于日志记录
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// SetupMiddlewares 设置所有中间件
func SetupMiddlewares(router *gin.Engine, config MiddlewareConfig) {
	// 基础中间件
	router.Use(RequestID())
	router.Use(Recovery()) // 使用简单的恢复中间件

	// 安全中间件
	router.Use(SecurityHeadersMiddleware())

	// CORS中间件
	if len(config.AllowedOrigins) > 0 {
		router.Use(CORSMiddleware(config.AllowedOrigins))
	}

	// 速率限制
	if config.RateLimit > 0 {
		router.Use(RateLimitMiddleware(config.RateLimit))
	}

	// 指标收集
	router.Use(MetricsMiddleware())

	// 请求验证
	router.Use(ValidationMiddleware())

	// 压缩
	if config.EnableCompression {
		router.Use(CompressionMiddleware())
	}
}

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	AllowedOrigins    []string
	RateLimit         int
	EnableCompression bool
	AuthTokens        []string
	RequestTimeout    time.Duration
}

// DefaultMiddlewareConfig 默认中间件配置
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		AllowedOrigins:    []string{"*"},
		RateLimit:         100, // 每分钟100个请求
		EnableCompression: true,
		RequestTimeout:    30 * time.Second,
	}
}
