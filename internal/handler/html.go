package handler

import (
	"MediaWarp/internal/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTMLPageHandler 处理HTML页面请求
func HTMLPageHandler(ctx *gin.Context) {
	pageName := ctx.Param("page")
	logging.Debug("请求HTML页面：", pageName)

	// 根据页面名称提供不同的数据
	switch pageName {
	case "dashboard":
		ctx.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title":   "MediaWarp - 控制面板",
			"heading": "MediaWarp控制面板",
			"content": "这是MediaWarp的控制面板页面。",
		})
	default:
		ctx.HTML(http.StatusOK, "index.html", gin.H{
			"title":   "MediaWarp - 首页",
			"heading": "欢迎使用MediaWarp",
			"content": "这是MediaWarp的首页。",
		})
	}
}
