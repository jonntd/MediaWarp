package handler

import (
	"MediaWarp/constants"
	"MediaWarp/internal/cache"
	"MediaWarp/internal/config"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/service"
	"MediaWarp/internal/service/emby"
	"MediaWarp/internal/service/jellyfin"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// OptimizedPlaybackHandler 优化后的播放处理器
type OptimizedPlaybackHandler struct {
	embyServer     *emby.EmbyServer
	jellyfinServer *jellyfin.Jellyfin
	cache          *cache.PlaybackInfoCache
	serverType     string // "emby" or "jellyfin"
}

// NewOptimizedPlaybackHandler 创建优化的播放处理器
func NewOptimizedPlaybackHandler(serverType string) *OptimizedPlaybackHandler {
	handler := &OptimizedPlaybackHandler{
		cache:      cache.GlobalPlaybackCache,
		serverType: serverType,
	}

	switch serverType {
	case "emby":
		handler.embyServer = emby.New(config.MediaServer.ADDR, config.MediaServer.AUTH)
	case "jellyfin":
		handler.jellyfinServer = jellyfin.New(config.MediaServer.ADDR, config.MediaServer.AUTH)
	}

	return handler
}

// HandlePlaybackInfo 优化的播放信息处理（消除重复调用）
func (h *OptimizedPlaybackHandler) HandlePlaybackInfo(ctx *gin.Context) {
	logging.Debug("======= OptimizedPlaybackHandler.HandlePlaybackInfo =======")

	// 提取媒体源ID
	mediaSourceID := h.extractMediaSourceID(ctx)
	if mediaSourceID == "" {
		logging.Warning("无法提取媒体源ID")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media source ID"})
		return
	}

	// 1. 检查完整播放信息缓存
	if cachedPlayback, found := h.cache.GetPlaybackInfo(mediaSourceID); found {
		logging.Info("播放信息缓存命中：", mediaSourceID)
		h.returnCachedPlaybackInfo(ctx, cachedPlayback)
		return
	}

	// 2. 获取媒体项信息（只调用一次！）
	itemInfo, err := h.getItemInfoOnce(mediaSourceID)
	if err != nil {
		logging.Warning("获取媒体项信息失败：", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get item info"})
		return
	}

	// 3. 识别Strm类型（只识别一次！）
	strmType, strmOption, err := h.getStrmTypeOnce(h.getItemPath(itemInfo))
	if err != nil {
		logging.Warning("识别Strm类型失败：", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to recognize strm type"})
		return
	}

	// 4. 构建优化的播放信息
	playbackInfo, err := h.buildOptimizedPlaybackInfo(itemInfo, strmType, strmOption)
	if err != nil {
		logging.Warning("构建播放信息失败：", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build playback info"})
		return
	}

	// 5. 缓存播放信息（15分钟TTL）
	h.cachePlaybackInfo(mediaSourceID, playbackInfo)

	// 6. 返回播放信息
	h.returnPlaybackInfo(ctx, playbackInfo)

	logging.Info("播放信息处理完成：", mediaSourceID)
}

// HandleVideoStream 优化的视频流处理（使用缓存，避免重复调用）
func (h *OptimizedPlaybackHandler) HandleVideoStream(ctx *gin.Context) {
	logging.Debug("======= OptimizedPlaybackHandler.HandleVideoStream =======")

	if ctx.Request.Method == http.MethodHead {
		// HEAD请求直接转发
		h.forwardToUpstream(ctx)
		return
	}

	// 提取媒体源ID
	mediaSourceID := ctx.Query("mediasourceid")
	if mediaSourceID == "" {
		logging.Warning("视频流请求缺少媒体源ID")
		h.forwardToUpstream(ctx)
		return
	}

	// 清理媒体源ID（移除前缀）
	cleanMediaSourceID := strings.Replace(mediaSourceID, "mediasource_", "", 1)

	// 1. 尝试从缓存获取媒体项信息
	itemInfo, err := h.getItemInfoFromCacheOrFetch(cleanMediaSourceID)
	if err != nil {
		logging.Warning("获取媒体项信息失败：", err)
		h.forwardToUpstream(ctx)
		return
	}

	// 2. 检查是否为Strm文件
	itemPath := h.getItemPath(itemInfo)
	if !strings.HasSuffix(strings.ToLower(itemPath), ".strm") {
		logging.Debug("非Strm文件，转发到上游服务器：", itemPath)
		h.forwardToUpstream(ctx)
		return
	}

	// 3. 从缓存获取Strm类型
	strmType, strmOption, err := h.getStrmTypeFromCacheOrRecognize(itemPath)
	if err != nil {
		logging.Warning("获取Strm类型失败：", err)
		h.forwardToUpstream(ctx)
		return
	}

	// 4. 处理不同类型的Strm文件
	h.handleStrmRedirect(ctx, strmType, strmOption, itemPath, mediaSourceID)
}

// getItemInfoOnce 获取媒体项信息（只调用一次，带缓存）
func (h *OptimizedPlaybackHandler) getItemInfoOnce(mediaSourceID string) (interface{}, error) {
	// 检查缓存
	if cachedItem, found := h.cache.GetItemInfo(mediaSourceID); found {
		logging.Info("媒体项信息缓存命中：", mediaSourceID)
		if h.serverType == "emby" {
			return cachedItem.EmbyItem, nil
		}
		return cachedItem.JellyfinItem, nil
	}

	// 缓存未命中，从上游获取
	logging.Info("媒体项信息缓存未命中，从上游获取：", mediaSourceID)

	var itemResponse interface{}
	var err error

	switch h.serverType {
	case "emby":
		itemResponse, err = h.embyServer.ItemsServiceQueryItem(mediaSourceID, 1, "Path,MediaSources")
		if err == nil {
			// 缓存Emby响应（30分钟TTL）
			h.cache.SetItemInfo(mediaSourceID, itemResponse.(*emby.EmbyResponse), nil, 30*time.Minute)
		}
	case "jellyfin":
		itemResponse, err = h.jellyfinServer.ItemsServiceQueryItem(mediaSourceID, 1, "Path,MediaSources")
		if err == nil {
			// 缓存Jellyfin响应（30分钟TTL）
			h.cache.SetItemInfo(mediaSourceID, nil, itemResponse.(*jellyfin.Response), 30*time.Minute)
		}
	default:
		return nil, fmt.Errorf("unsupported server type: %s", h.serverType)
	}

	return itemResponse, err
}

// getStrmTypeOnce 识别Strm类型（只识别一次，带缓存）
func (h *OptimizedPlaybackHandler) getStrmTypeOnce(filePath string) (constants.StrmFileType, interface{}, error) {
	// 检查缓存
	if cachedStrm, found := h.cache.GetStrmType(filePath); found {
		logging.Info("Strm类型缓存命中：", filePath)
		return cachedStrm.Type, cachedStrm.Option, nil
	}

	// 缓存未命中，重新识别
	logging.Info("Strm类型缓存未命中，重新识别：", filePath)

	strmType, option, _ := recgonizeStrmFileType(filePath)

	// 缓存结果（1小时TTL，文件路径很少变化）
	h.cache.SetStrmType(filePath, strmType, option, 1*time.Hour)

	return strmType, option, nil
}

// getItemInfoFromCacheOrFetch 从缓存或重新获取媒体项信息
func (h *OptimizedPlaybackHandler) getItemInfoFromCacheOrFetch(mediaSourceID string) (interface{}, error) {
	// 优先从缓存获取
	if cachedItem, found := h.cache.GetItemInfo(mediaSourceID); found {
		if h.serverType == "emby" {
			return cachedItem.EmbyItem, nil
		}
		return cachedItem.JellyfinItem, nil
	}

	// 缓存未命中，重新获取
	return h.getItemInfoOnce(mediaSourceID)
}

// getStrmTypeFromCacheOrRecognize 从缓存或重新识别Strm类型
func (h *OptimizedPlaybackHandler) getStrmTypeFromCacheOrRecognize(filePath string) (constants.StrmFileType, interface{}, error) {
	// 优先从缓存获取
	if cachedStrm, found := h.cache.GetStrmType(filePath); found {
		return cachedStrm.Type, cachedStrm.Option, nil
	}

	// 缓存未命中，重新识别
	return h.getStrmTypeOnce(filePath)
}

// handleStrmRedirect 处理Strm文件重定向
func (h *OptimizedPlaybackHandler) handleStrmRedirect(ctx *gin.Context, strmType constants.StrmFileType, strmOption interface{}, itemPath, mediaSourceID string) {
	switch strmType {
	case constants.HTTPStrm:
		// HTTPStrm直接重定向
		logging.Info("HTTPStrm重定向至：", itemPath)
		ctx.Redirect(http.StatusFound, itemPath)

	case constants.AlistStrm:
		// AlistStrm需要获取下载链接
		redirectURL, err := h.getAlistDownloadURL(itemPath, strmOption.(string))
		if err != nil {
			logging.Warning("获取Alist下载链接失败：", err)
			h.forwardToUpstream(ctx)
			return
		}

		logging.Info("AlistStrm重定向至：", redirectURL)
		ctx.Redirect(http.StatusFound, redirectURL)

	default:
		// 未知类型，转发到上游
		logging.Debug("未知Strm类型，转发到上游服务器")
		h.forwardToUpstream(ctx)
	}
}

// getAlistDownloadURL 获取Alist下载链接（带缓存）
func (h *OptimizedPlaybackHandler) getAlistDownloadURL(filePath, alistServerAddr string) (string, error) {
	// 检查缓存
	if cachedLink, found := h.cache.GetAlistLink(filePath); found {
		logging.Info("Alist链接缓存命中：", filePath)
		if config.AlistStrm.RawURL {
			return cachedLink.RawURL, nil
		}

		downloadURL := fmt.Sprintf("%s/d%s", alistServerAddr, filePath)
		if cachedLink.Sign != "" {
			downloadURL += "?sign=" + cachedLink.Sign
		}
		return downloadURL, nil
	}

	// 缓存未命中，从Alist获取
	logging.Info("Alist链接缓存未命中，从Alist获取：", filePath)

	alistServer, err := service.GetAlistServer(alistServerAddr)
	if err != nil {
		return "", fmt.Errorf("获取AlistServer失败: %w", err)
	}

	fsGetData, err := alistServer.FsGet(filePath)
	if err != nil {
		return "", fmt.Errorf("请求FsGet失败: %w", err)
	}

	// 缓存链接信息（5分钟TTL，考虑签名过期）
	h.cache.SetAlistLink(filePath, "", fsGetData.Sign, fsGetData.RawURL, 5*time.Minute)

	// 构建下载URL
	var redirectURL string
	if config.AlistStrm.RawURL {
		redirectURL = fsGetData.RawURL
	} else {
		redirectURL = fmt.Sprintf("%s/d%s", alistServerAddr, filePath)
		if fsGetData.Sign != "" {
			redirectURL += "?sign=" + fsGetData.Sign
		}
	}

	return redirectURL, nil
}

// 辅助方法
func (h *OptimizedPlaybackHandler) extractMediaSourceID(ctx *gin.Context) string {
	// 从URL路径提取媒体源ID
	path := ctx.Request.URL.Path
	if strings.Contains(path, "/Items/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "Items" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}

func (h *OptimizedPlaybackHandler) getItemPath(itemInfo interface{}) string {
	switch h.serverType {
	case "emby":
		if embyItem, ok := itemInfo.(*emby.EmbyResponse); ok && len(embyItem.Items) > 0 {
			return *embyItem.Items[0].Path
		}
	case "jellyfin":
		if jellyfinItem, ok := itemInfo.(*jellyfin.Response); ok && len(jellyfinItem.Items) > 0 {
			return *jellyfinItem.Items[0].Path
		}
	}
	return ""
}

func (h *OptimizedPlaybackHandler) forwardToUpstream(ctx *gin.Context) {
	// 转发到上游服务器的逻辑
	mediaServerHandler := GetMediaServer()
	mediaServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
}

// buildOptimizedPlaybackInfo 构建优化的播放信息
func (h *OptimizedPlaybackHandler) buildOptimizedPlaybackInfo(itemInfo interface{}, strmType constants.StrmFileType, strmOption interface{}) (interface{}, error) {
	// 这里实现播放信息的构建逻辑
	// 根据服务器类型和Strm类型进行相应的处理
	// 具体实现需要根据现有的ModifyPlaybackInfo逻辑进行适配

	// 暂时返回原始信息，后续需要完善
	return itemInfo, nil
}

func (h *OptimizedPlaybackHandler) cachePlaybackInfo(mediaSourceID string, playbackInfo interface{}) {
	// 缓存播放信息的逻辑
	switch h.serverType {
	case "emby":
		if embyResponse, ok := playbackInfo.(*emby.PlaybackInfoResponse); ok {
			h.cache.SetPlaybackInfo(mediaSourceID, embyResponse, nil, 15*time.Minute)
		}
	case "jellyfin":
		if jellyfinResponse, ok := playbackInfo.(*jellyfin.PlaybackInfoResponse); ok {
			h.cache.SetPlaybackInfo(mediaSourceID, nil, jellyfinResponse, 15*time.Minute)
		}
	}
}

func (h *OptimizedPlaybackHandler) returnCachedPlaybackInfo(ctx *gin.Context, cachedPlayback *cache.CachedPlaybackInfo) {
	if h.serverType == "emby" && cachedPlayback.EmbyResponse != nil {
		ctx.JSON(http.StatusOK, cachedPlayback.EmbyResponse)
	} else if h.serverType == "jellyfin" && cachedPlayback.JellyfinResponse != nil {
		ctx.JSON(http.StatusOK, cachedPlayback.JellyfinResponse)
	} else {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid cached playback info"})
	}
}

func (h *OptimizedPlaybackHandler) returnPlaybackInfo(ctx *gin.Context, playbackInfo interface{}) {
	ctx.JSON(http.StatusOK, playbackInfo)
}
