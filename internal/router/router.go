package router

import (
	"MediaWarp/constants"
	"MediaWarp/internal/config"
	"MediaWarp/internal/handler"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/middleware"
	"MediaWarp/static"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine {
	ginR := gin.New()
	ginR.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.QueryCaseInsensitive(),
		middleware.SetRefererPolicy(constants.SameOrigin),
	)

	if config.ClientFilter.Enable {
		ginR.Use(middleware.ClientFilter())
		logging.Info("客户端过滤中间件已启用")
	} else {
		logging.Info("客户端过滤中间件未启用")
	}

	// 加载HTML模板
	templateFS, err := fs.Sub(static.EmbeddedStaticAssets, "templates")
	if err != nil {
		logging.Error("加载HTML模板失败：", err)
	} else {
		templ := template.Must(template.New("").ParseFS(templateFS, "*.html"))
		ginR.SetHTMLTemplate(templ)
	}

	// 添加静态文件服务
	staticFS, err := fs.Sub(static.EmbeddedStaticAssets, "emby-crx/static")
	if err != nil {
		logging.Error("加载静态资源失败：", err)
	} else {
		ginR.StaticFS("/MediaWarp/static/emby-crx/static", http.FS(staticFS))
	}

	ginR.GET("/login", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "login.html", gin.H{})
	})
	mediawarpRouter := ginR.GET("/page/:page", handler.HTMLPageHandler)
	{
		mediawarpRouter.Any("/version", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, config.Version())
		})

	}
	handler.SyncFilesRouter(ginR)
	ginR.NoRoute(RegexpRouterHandler)
	return ginR
}

// 正则表达式路由处理器
//
// 从媒体服务器处理结构体中获取正则路由规则
// 依次匹配请求, 找到对应的处理器
func RegexpRouterHandler(ctx *gin.Context) {
	mediaServerHandler := handler.GetMediaServer()

	for _, rule := range mediaServerHandler.GetRegexpRouteRules() {
		if rule.Regexp.MatchString(ctx.Request.URL.Path) { // 不带查询参数的字符串：/emby/Items/54/Images/Primary
			logging.Debugf("URL: %s 匹配成功 -> %s", ctx.Request.URL.Path, rule.Regexp.String())
			rule.Handler(ctx)
			return
		}
	}

	// 未匹配路由
	mediaServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
}
