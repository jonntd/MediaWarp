package router

import (
	_115 "MediaWarp/115"
	"MediaWarp/controllers"
	"MediaWarp/core"
	"MediaWarp/middleware"
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

var DriveClient *_115.DriveClient
var config = core.GetConfig()
var staticFiles embed.FS

func InitRouter() *gin.Engine {
	ginR := gin.New()
	// ginR.Use(middleware.LogRawRequest())
	DriveClient = _115.MustNew115DriveClient(config.Cookie)
	controllers.TaskCron.Start()
	// ginR.Static("/static", "./static")
	ginR.StaticFS("/static", http.FS(staticFiles))

	ginR.Use(middleware.QueryCaseInsensitive())
	ginR.Use(middleware.LogMiddleware())
	controllers.TaskCronRouter(ginR)
	controllers.SyncFilesRouter(ginR)

	// UserLibraryService
	registerRoutes(ginR, "/Users/:userId/Items", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Resume", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Latest", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Views", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/:itemId", controllers.ItemsHandler, http.MethodGet)
	// ItemsService
	registerRoutes(ginR, "/Items/:itemId/PlaybackInfo", controllers.PlaybackInfoHandler, http.MethodGet)
	registerRoutes(ginR, "/Items/:itemId/PlaybackInfo", controllers.PlaybackInfoHandler, http.MethodPost)
	// registerRoutes(ginR, "/Sync/*path", controllers.MediaFileSyncHandler, http.MethodGet)
	// VideoService
	// registerRoutes(ginR, "/Videos/:itemId/:name", controllers.VideosHandler, http.MethodGet)
	registerRoutes(ginR, "/Videos/:itemId/:name", func(c *gin.Context) {
		controllers.StreamHandler(c, DriveClient)
	}, http.MethodGet)
	registerRoutes(ginR, "/videos/:videoId/stream.strm", func(c *gin.Context) {
		controllers.StreamHandler(c, DriveClient)
	}, http.MethodGet)

	ginR.GET("/web/modules/htmlvideoplayer/basehtmlplayer.js", controllers.ModifyBaseHtmlPlayerHandler)
	ginR.NoRoute(controllers.DefaultHandler)

	return ginR
}

// 注册路由
func registerRoutes(router *gin.Engine, path string, handler gin.HandlerFunc, method string) {
	embyPath := "/emby" + path
	switch method {
	case http.MethodGet:
		router.GET(path, handler)
		router.GET(embyPath, handler)
	case http.MethodPost:
		router.POST(path, handler)
		router.POST(embyPath, handler)
	}
}
