package middleware

import (
	"MediaWarp/internal/logging"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 常见的搜索引擎和爬虫User-Agent
var botUserAgents = []string{
	"googlebot",
	"bingbot",
	"slurp",
	"duckduckbot",
	"baiduspider",
	"yandexbot",
	"facebookexternalhit",
	"twitterbot",
	"linkedinbot",
	"whatsapp",
	"telegrambot",
	"discordbot",
	"applebot",
	"amazonbot",
	"msnbot",
	"yahoo",
	"crawler",
	"spider",
	"bot",
	"scraper",
	"curl",
	"wget",
	"python-requests",
	"go-http-client",
	"java",
	"php",
	"perl",
	"ruby",
	"node",
	"axios",
	"fetch",
}

// 可疑的请求特征
var suspiciousPatterns = []string{
	"scan",
	"probe",
	"test",
	"check",
	"monitor",
	"audit",
	"security",
	"vulnerability",
}

// AntiBotMiddleware 防爬虫中间件
func AntiBotMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 添加安全头防止索引和缓存
		c.Header("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")

		userAgent := strings.ToLower(c.GetHeader("User-Agent"))

		// 检查是否为已知的爬虫
		for _, botUA := range botUserAgents {
			if strings.Contains(userAgent, botUA) {
				logging.Warning("检测到爬虫访问", "user_agent", userAgent, "ip", c.ClientIP(), "path", c.Request.URL.Path)

				// 返回robots.txt内容而不是403，更友好
				c.Header("Content-Type", "text/plain")
				c.String(http.StatusOK, "User-agent: *\nDisallow: /")
				c.Abort()
				return
			}
		}

		// 检查可疑的请求模式
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(userAgent, pattern) {
				logging.Warning("检测到可疑请求", "user_agent", userAgent, "ip", c.ClientIP(), "path", c.Request.URL.Path)
				c.String(http.StatusForbidden, "Access denied")
				c.Abort()
				return
			}
		}

		// 检查空User-Agent（很多爬虫会这样）
		if userAgent == "" {
			logging.Warning("检测到空User-Agent", "ip", c.ClientIP(), "path", c.Request.URL.Path)
			c.String(http.StatusBadRequest, "User-Agent required")
			c.Abort()
			return
		}

		c.Next()
	}
}
