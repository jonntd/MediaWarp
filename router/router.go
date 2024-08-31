package router

import (
	_115 "MediaWarp/115"
	"MediaWarp/controllers"
	"MediaWarp/core"
	"MediaWarp/middleware"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

var DriveClient *_115.DriveClient
var config = core.GetConfig()

func make_config() {
	remoteName := config.Remote
	remoteType := config.Remote
	cookie := config.Cookie
	pacerMinSleep := "0.333"
	cmdDelete := exec.Command("rclone", "config", "delete", remoteName)
	if output, err := cmdDelete.CombinedOutput(); err != nil {
		fmt.Printf("Failed to delete existing config: %s\nOutput: %s", err, string(output))
	}

	cmd := exec.Command("rclone", "config", "create", remoteName, remoteType,
		"cookie", cookie,
		"pacer_min_sleep", pacerMinSleep,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to execute command: %s\nOutput: %s", err, string(output))
	}
	fmt.Printf("Command output: %s", string(output))
}

func InitRouter() *gin.Engine {
	ginR := gin.New()
	// ginR.Use(middleware.LogRawRequest())
	DriveClient = _115.MustNew115DriveClient(config.Cookie)
	make_config()
	core.TaskCron.Start()

	ginR.Static("/static", "./static")
	ginR.Use(middleware.QueryCaseInsensitive())
	ginR.Use(middleware.LogMiddleware())
	core.SetupRouter(ginR)

	// UserLibraryService
	registerRoutes(ginR, "/Users/:userId/Items", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Resume", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Latest", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/Views", controllers.DefaultHandler, http.MethodGet)
	registerRoutes(ginR, "/Users/:userId/Items/:itemId", controllers.ItemsHandler, http.MethodGet)
	// ItemsService
	registerRoutes(ginR, "/Items/:itemId/PlaybackInfo", controllers.PlaybackInfoHandler, http.MethodGet)
	registerRoutes(ginR, "/Items/:itemId/PlaybackInfo", controllers.PlaybackInfoHandler, http.MethodPost)
	registerRoutes(ginR, "/Sync/*path", controllers.MediaFileSyncHandler, http.MethodGet)
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
