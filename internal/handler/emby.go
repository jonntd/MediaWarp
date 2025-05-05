package handler

import (
	_115 "MediaWarp/115"
	"MediaWarp/constants"
	"MediaWarp/internal/config"
	"MediaWarp/internal/logging"
	"MediaWarp/internal/service/emby"
	"MediaWarp/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ApiResponse struct {
	State   bool                `json:"state"`   // Indicates success or failure
	Message string              `json:"message"` // Optional message
	Code    int                 `json:"code"`    // Status code
	Data    map[string]FileInfo `json:"data"`    // Map where keys are file IDs (strings) and values are FileInfo objects
}

// FileInfo represents the details of a single file.
// The key for this object in the parent map (ApiResponse.Data) is the file ID.
type FileInfo struct {
	FileName string      `json:"file_name"` // Name of the file
	FileSize int64       `json:"file_size"` // Size of the file in bytes (using int64 for potentially large files)
	PickCode string      `json:"pick_code"` // File pick code (extraction code)
	SHA1     string      `json:"sha1"`      // SHA1 hash of the file content
	URL      DownloadURL `json:"url"`       // Object containing the download URL
}
type DownloadURL struct {
	URL string `json:"url"` // The file download address
}

var DriveClient *_115.DriveClient
var redirectURL string

func init() {
	// cookie := "UID=341794039_M1_1733655944; CID=6703a177cb0e92e9908d44a49e3032b2; SEID=a9420ed11bc9d9ac818e024086b17f5112dab42fa84c6e4669072aa009cb435b1eb50b836bba16573b75d50bf43e1e2bd034bb02b9c147347ff2734d; KID=c9416be503606bbcb5eb329520fe5c5d"
	// DriveClient = _115.MustNew115DriveClient(cookie)
}

func convertToLinuxPath(windowsPath string) string {
	linuxPath := strings.ReplaceAll(windowsPath, "\\", "/")
	return linuxPath
}

func ensureLeadingSlash(alistPath string) string {
	if !strings.HasPrefix(alistPath, "/") {
		alistPath = "/" + alistPath // 不是以 / 开头，加上 /
	}

	alistPath = convertToLinuxPath(alistPath)
	return alistPath
}

// Emby服务器处理器
type EmbyServerHandler struct {
	server      *emby.EmbyServer       // Emby 服务器
	routerRules []RegexpRouteRule      // 正则路由规则
	proxy       *httputil.ReverseProxy // 反向代理
}

// 初始化
func NewEmbyServerHandler(addr string, apiKey string) (*EmbyServerHandler, error) {
	var embyServerHandler = EmbyServerHandler{}
	embyServerHandler.server = emby.New(addr, apiKey)
	target, err := url.Parse(embyServerHandler.server.GetEndpoint())
	if err != nil {
		return nil, err
	}
	embyServerHandler.proxy = httputil.NewSingleHostReverseProxy(target)

	{ // 初始化路由规则
		embyServerHandler.routerRules = []RegexpRouteRule{
			{
				Regexp:  constants.EmbyRegexp.Router.VideosHandler,
				Handler: embyServerHandler.VideosHandler,
			},
			{
				Regexp: constants.EmbyRegexp.Router.ModifyPlaybackInfo,
				Handler: responseModifyCreater(
					&httputil.ReverseProxy{Director: embyServerHandler.proxy.Director},
					embyServerHandler.ModifyPlaybackInfo,
				),
			},
			{
				Regexp: constants.EmbyRegexp.Router.ModifyBaseHtmlPlayer,
				Handler: responseModifyCreater(
					&httputil.ReverseProxy{Director: embyServerHandler.proxy.Director},
					embyServerHandler.ModifyBaseHtmlPlayer,
				),
			},
			{
				Regexp:  constants.EmbyRegexp.Router.StreamStrmHandler,
				Handler: embyServerHandler.VideosHandler,
			},
		}

		if config.Web.Enable {
			if config.Web.Index || config.Web.Head != "" || config.Web.ExternalPlayerUrl || config.Web.VideoTogether {
				embyServerHandler.routerRules = append(embyServerHandler.routerRules,
					RegexpRouteRule{
						Regexp: constants.EmbyRegexp.Router.ModifyIndex,
						Handler: responseModifyCreater(
							&httputil.ReverseProxy{Director: embyServerHandler.proxy.Director},
							embyServerHandler.ModifyIndex,
						),
					},
				)
			}
		}
		if config.Subtitle.Enable && config.Subtitle.SRT2ASS {
			embyServerHandler.routerRules = append(embyServerHandler.routerRules,
				RegexpRouteRule{
					Regexp: constants.EmbyRegexp.Router.ModifySubtitles,
					Handler: responseModifyCreater(
						&httputil.ReverseProxy{Director: embyServerHandler.proxy.Director},
						embyServerHandler.ModifySubtitles,
					),
				},
			)
		}
	}
	return &embyServerHandler, nil
}

// 转发请求至上游服务器
func (embyServerHandler *EmbyServerHandler) ReverseProxy(rw http.ResponseWriter, req *http.Request) {
	embyServerHandler.proxy.ServeHTTP(rw, req)
}

// 正则路由表
func (embyServerHandler *EmbyServerHandler) GetRegexpRouteRules() []RegexpRouteRule {
	return embyServerHandler.routerRules
}

// 修改播放信息请求
//
// /Items/:itemId/PlaybackInfo
// 强制将 HTTPStrm 设置为支持直链播放和转码、AlistStrm 设置为支持直链播放并且禁止转码
func (embyServerHandler *EmbyServerHandler) ModifyPlaybackInfo(rw *http.Response) error {
	logging.Debug("=======  ModifyPlaybackInfo ======= ")

	defer rw.Body.Close()
	body, err := readBody(rw)
	if err != nil {
		logging.Warning("读取 Body 出错：", err)
		return err
	}

	var playbackInfoResponse emby.PlaybackInfoResponse
	if err = json.Unmarshal(body, &playbackInfoResponse); err != nil {
		logging.Warning("解析 emby.PlaybackInfoResponse Json 错误：", err)
		return err
	}

	for index, mediasource := range playbackInfoResponse.MediaSources {
		logging.Debug("请求 ItemsServiceQueryItem：" + *mediasource.ID)
		itemResponse, err := embyServerHandler.server.ItemsServiceQueryItem(strings.Replace(*mediasource.ID, "mediasource_", "", 1), 1, "Path,MediaSources") // 查询 item 需要去除前缀仅保留数字部分
		if err != nil {
			logging.Warning("请求 ItemsServiceQueryItem 失败：", err)
			continue
		}
		item := itemResponse.Items[0]
		strmFileType, _, _ := recgonizeStrmFileType(*item.Path)
		switch strmFileType {
		case constants.HTTPStrm: // HTTPStrm 设置支持直链播放并且支持转码
			if !config.HTTPStrm.TransCode {
				*playbackInfoResponse.MediaSources[index].SupportsDirectPlay = true
				*playbackInfoResponse.MediaSources[index].SupportsDirectStream = true
				playbackInfoResponse.MediaSources[index].TranscodingURL = nil
				playbackInfoResponse.MediaSources[index].TranscodingSubProtocol = nil
				playbackInfoResponse.MediaSources[index].TranscodingContainer = nil
				if mediasource.DirectStreamURL != nil {
					apikeypair, err := utils.ResolveEmbyAPIKVPairs(*mediasource.DirectStreamURL)
					if err != nil {
						logging.Warning("解析API键值对失败：", err)
						continue
					}
					directStreamURL := fmt.Sprintf("/videos/%s/stream?MediaSourceId=%s&Static=true&%s", *mediasource.ItemID, *mediasource.ID, apikeypair)
					playbackInfoResponse.MediaSources[index].DirectStreamURL = &directStreamURL
					logging.Info(*mediasource.Name, "强制禁止转码，直链播放链接为:", directStreamURL)
				}
			}

		case constants.AlistStrm: // AlistStm 设置支持直链播放并且禁止转码
			if !config.AlistStrm.TransCode {
				*playbackInfoResponse.MediaSources[index].SupportsDirectPlay = true
				*playbackInfoResponse.MediaSources[index].SupportsDirectStream = true
				*playbackInfoResponse.MediaSources[index].SupportsTranscoding = false
				playbackInfoResponse.MediaSources[index].TranscodingURL = nil
				playbackInfoResponse.MediaSources[index].TranscodingSubProtocol = nil
				playbackInfoResponse.MediaSources[index].TranscodingContainer = nil
				apikeypair, err := utils.ResolveEmbyAPIKVPairs(*mediasource.DirectStreamURL)
				if err != nil {
					logging.Warning("解析API键值对失败：", err)
					continue
				}
				directStreamURL := fmt.Sprintf("/videos/%s/stream?MediaSourceId=%s&Static=true&%s", *mediasource.ItemID, *mediasource.ID, apikeypair)
				playbackInfoResponse.MediaSources[index].DirectStreamURL = &directStreamURL
				container := strings.TrimPrefix(path.Ext(*mediasource.Path), ".")
				playbackInfoResponse.MediaSources[index].Container = &container
				logging.Info(*mediasource.Name, "强制禁止转码，直链播放链接为:", directStreamURL, "，容器为: %s", container)
			} else {
				logging.Info(*mediasource.Name, "保持原有转码设置")
			}

			// if playbackInfoResponse.MediaSources[index].Size == nil {
			// 	alistServer, err := service.GetAlistServer(opt.(string))
			// 	if err != nil {
			// 		logging.Warning("获取 AlistServer 失败：", err)
			// 		continue
			// 	}
			// 	fsGetData, err := alistServer.FsGet(*mediasource.Path)
			// 	if err != nil {
			// 		logging.Warning("请求 FsGet 失败：", err)
			// 		continue
			// 	}
			// 	playbackInfoResponse.MediaSources[index].Size = &fsGetData.Size
			// 	logging.Info(*mediasource.Name, "设置文件大小为:", fsGetData.Size)
			// }
		}
	}

	body, err = json.Marshal(playbackInfoResponse)
	if err != nil {
		logging.Warning("序列化 emby.PlaybackInfoResponse Json 错误：", err)
		return err
	}

	rw.Header.Set("Content-Type", "application/json") // 更新 Content-Type 头
	return updateBody(rw, body)
}

// 视频流处理器
//
// 支持播放本地视频、重定向 HttpStrm、AlistStrm
func (embyServerHandler *EmbyServerHandler) VideosHandler(ctx *gin.Context) {
	logging.Debug("======= VideosHandler ======= ")

	if ctx.Request.Method == http.MethodHead { // 不额外处理 HEAD 请求
		embyServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
		logging.Debug("VideosHandler 不处理 HEAD 请求，转发至上游服务器")
		return
	}

	orginalPath := ctx.Request.URL.Path
	logging.Debug("orginalPath:", orginalPath)

	matches := constants.EmbyRegexp.Others.VideoRedirectReg.FindStringSubmatch(orginalPath)
	if len(matches) == 2 {
		redirectPath := fmt.Sprintf("/videos/%s/stream", matches[0])
		logging.Debug(orginalPath + " 重定向至：" + redirectPath)
		ctx.Redirect(http.StatusFound, redirectPath)
		return
	}

	// EmbyServer <= 4.8 ====> mediaSourceID = 343121
	// EmbyServer >= 4.9 ====> mediaSourceID = mediasource_31
	mediaSourceID := ctx.Query("mediasourceid")

	logging.Debug("请求 ItemsServiceQueryItem：", mediaSourceID)
	itemResponse, err := embyServerHandler.server.ItemsServiceQueryItem(strings.Replace(mediaSourceID, "mediasource_", "", 1), 1, "Path,MediaSources") // 查询 item 需要去除前缀仅保留数字部分
	if err != nil {
		logging.Warning("请求 ItemsServiceQueryItem 失败：", err)
		embyServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
		return
	}
	item := itemResponse.Items[0]
	if !strings.HasSuffix(strings.ToLower(*item.Path), ".strm") { // 不是 Strm 文件
		logging.Debug("播放本地视频：" + *item.Path + "，不进行处理")
		embyServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
		return
	}
	strmFileType, opt, srerveAdd := recgonizeStrmFileType(*item.Path)
	logging.Debug("请求 strmFileType:", strmFileType)
	for _, mediasource := range item.MediaSources {
		if *mediasource.ID == mediaSourceID { // EmbyServer >= 4.9 返回的ID带有前缀mediasource_
			switch strmFileType {
			case constants.HTTPStrm:
				if *mediasource.Protocol == emby.HTTP {
					logging.Info("HTTPStrm 重定向至：", *mediasource.Path)
					ctx.Redirect(http.StatusFound, *mediasource.Path)
				}
				return
			case constants.AlistStrm: // 无需判断 *mediasource.Container 是否以Strm结尾，当 AlistStrm 存储的位置有对应的文件时，*mediasource.Container 会被设置为文件后缀
				// alistServerAddr := opt.(string)
				// alistServer, err := service.GetAlistServer(alistServerAddr)
				// if err != nil {
				// 	logging.Warning("获取 AlistServer 失败：", err)
				// 	return
				// }
				// fsGetData, err := alistServer.FsGet(*mediasource.Path)
				// if err != nil {
				// 	logging.Warning("请求 FsGet 失败：", err)
				// 	return
				// }
				// var redirectURL string
				// if config.AlistStrm.RawURL {
				// 	redirectURL = fsGetData.RawURL
				// } else {
				// 	redirectURL = fmt.Sprintf("%s/d%s", alistServerAddr, *mediasource.Path)
				// 	if fsGetData.Sign != "" {
				// 		redirectURL += "?sign=" + fsGetData.Sign
				// 	}
				// }

				desiredPath := strings.Replace(*mediasource.Path, opt.(string), "", 1)
				desiredPath = ensureLeadingSlash(desiredPath)
				logging.Debug("请求 desiredPath:", desiredPath)
				logging.Debug("请求 opt.(string):", opt.(string))

				// Type 1
				downloadurl := fmt.Sprintf("%s:%s", srerveAdd, desiredPath)
				userAgent := fmt.Sprintf("%s", ctx.Request.Header.Get("User-Agent"))
				logging.Debug("downloadurl:", downloadurl)
				cacheKey := downloadurl + "|" + userAgent

				// 尝试从缓存获取URL
				cacheMutex.RLock()
				cachedItem, exists := redirectURLCache[cacheKey]
				cacheMutex.RUnlock()

				// 检查缓存是否存在且未过期
				if exists && time.Now().Before(cachedItem.ExpireTime) {
					logging.Info("从缓存获取重定向URL：", cachedItem.URL)
					redirectURL = cachedItem.URL
				} else {
					// 缓存不存在或已过期，执行rclone命令获取URL
					cmd := exec.CommandContext(ctx, "rclone", "backend", "getdownloadurlua", downloadurl, userAgent)
					var stdoutBuf, stderrBuf bytes.Buffer
					cmd.Stdout = &stdoutBuf
					cmd.Stderr = &stderrBuf
					fmt.Printf("Running command: %s\n", cmd.String())
					err := cmd.Run()
					if err != nil {
						logging.Warning("执行 rclone command 失败：", err)
						return
					}
					logging.Info("stdoutBuf：", stdoutBuf.String())
					redirectURL = strings.TrimSpace(stdoutBuf.String())
					if redirectURL == "" {
						return
					}
					// 将新获取的URL存入缓存
					cacheMutex.Lock()
					redirectURLCache[cacheKey] = CacheItem{
						URL:        redirectURL,
						ExpireTime: time.Now().Add(defaultCacheTime),
					}
					cacheMutex.Unlock()
					logging.Info("缓存重定向URL，过期时间：", time.Now().Add(defaultCacheTime))
				}
				logging.Info("AlistStrm 重定向至：", fmt.Sprintf("==%s==", redirectURL))
				ctx.Redirect(http.StatusFound, redirectURL)
				// Type 2
				// logging.Info("302重定向：", *mediasource.Path)
				// files, err := DriveClient.GetFile(desiredPath)
				// if err != nil {
				// 	return
				// }
				// userAgent := ctx.Request.Header.Get("User-Agent")
				// down_url, err := DriveClient.GetFileURL(files, userAgent)
				// if err != nil {
				// 	return
				// }
				// logging.Info("down_url：", down_url)

				// Type 3
				// 构建请求115 API获取下载链接
				// client := &http.Client{}
				// req, err := http.NewRequest("POST", "https://proapi.115.com/open/ufile/downurl",
				// 	strings.NewReader("pick_code=cv6i4y9laun1ddxq3"))
				// if err != nil {
				// 	logging.Warning("创建请求失败:", err)
				// 	return
				// }

				// // 设置请求头
				// req.Header.Set("Authorization", "Bearer tszs.afb9ac7a16203862d639e17a881315cf.c70a71ce40efe186e0d84b6a5441ec9dff80219a94ca2efd932e1547978df049")
				// req.Header.Set("User-Agent", ctx.Request.Header.Get("User-Agent"))
				// req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				// // 发送请求并处理响应
				// res, err := client.Do(req)
				// if err != nil {
				// 	logging.Warning("请求失败:", err)
				// 	return
				// }
				// defer res.Body.Close()

				// // 解析响应
				// var response ApiResponse
				// if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
				// 	logging.Warning("解析响应失败:", err)
				// 	return
				// }

				// // 获取下载URL并重定向
				// for _, downInfo := range response.Data {
				// 	if downInfo != (FileInfo{}) {
				// 		logging.Info("获取到下载URL:", downInfo.URL.URL)
				// 		ctx.Redirect(http.StatusFound, downInfo.URL.URL)
				// 		return
				// 	}
				// }
				return
			case constants.UnknownStrm:
				embyServerHandler.ReverseProxy(ctx.Writer, ctx.Request)
				return
			}
		}
	}
}

// 修改字幕
//
// 将 SRT 字幕转 ASS
func (embyServerHandler *EmbyServerHandler) ModifySubtitles(rw *http.Response) error {
	defer rw.Body.Close()
	subtitile, err := readBody(rw) // 读取字幕文件
	if err != nil {
		logging.Warning("读取原始字幕 Body 出错：", err)
		return err
	}

	if utils.IsSRT(subtitile) { // 判断是否为 SRT 格式
		logging.Info("字幕文件为 SRT 格式")
		if config.Subtitle.SRT2ASS {
			logging.Info("已将 SRT 字幕已转为 ASS 格式")
			assSubtitle := utils.SRT2ASS(subtitile, config.Subtitle.ASSStyle)
			return updateBody(rw, assSubtitle)
		}
	}
	return nil
}

// 修改 basehtmlplayer.js
//
// 用于修改播放器 JS，实现跨域播放 Strm 文件（302 重定向）
func (embyServerHandler *EmbyServerHandler) ModifyBaseHtmlPlayer(rw *http.Response) error {
	defer rw.Body.Close()
	body, err := readBody(rw)
	if err != nil {
		return err
	}

	body = bytes.ReplaceAll(body, []byte(`mediaSource.IsRemote&&"DirectPlay"===playMethod?null:"anonymous"`), []byte("null")) // 修改响应体
	return updateBody(rw, body)

}

// 修改首页函数
func (embyServerHandler *EmbyServerHandler) ModifyIndex(rw *http.Response) error {
	var (
		htmlFilePath string = path.Join(config.CostomDir(), "index.html")
		htmlContent  []byte
		addHEAD      []byte
		err          error
	)

	defer rw.Body.Close()  // 无论哪种情况，最终都要确保原 Body 被关闭，避免内存泄漏
	if !config.Web.Index { // 从上游获取响应体
		if htmlContent, err = readBody(rw); err != nil {
			return err
		}
	} else { // 从本地文件读取index.html
		if htmlContent, err = os.ReadFile(htmlFilePath); err != nil {
			logging.Warning("读取文件内容出错，错误信息：", err)
			return err
		}
	}

	if config.Web.Head != "" { // 用户自定义HEAD
		addHEAD = append(addHEAD, []byte(config.Web.Head+"\n")...)
	}
	if config.Web.ExternalPlayerUrl { // 外部播放器
		addHEAD = append(addHEAD, []byte(`<script src="/MediaWarp/static/embyExternalUrl/embyWebAddExternalUrl/embyLaunchPotplayer.js"></script>`+"\n")...)
	}
	if config.Web.Crx { // crx 美化
		addHEAD = append(addHEAD, []byte(`<link rel="stylesheet" id="theme-css" href="/MediaWarp/static/emby-crx/static/css/style.css" type="text/css" media="all" />
    <script src="/MediaWarp/static/emby-crx/static/js/common-utils.js"></script>
    <script src="/MediaWarp/static/emby-crx/static/js/jquery-3.6.0.min.js"></script>
    <script src="/MediaWarp/static/emby-crx/static/js/md5.min.js"></script>
    <script src="/MediaWarp/static/emby-crx/content/main.js"></script>`+"\n")...)
	}
	if config.Web.ActorPlus { // 过滤没有头像的演员和制作人员
		addHEAD = append(addHEAD, []byte(`<script src="/MediaWarp/static/emby-web-mod/actorPlus/actorPlus.js"></script>`+"\n")...)
	}
	if config.Web.FanartShow { // 显示同人图（fanart图）
		addHEAD = append(addHEAD, []byte(`<script src="/MediaWarp/static/emby-web-mod/fanart_show/fanart_show.js"></script>`+"\n")...)
	}
	if config.Web.Danmaku { // 弹幕
		addHEAD = append(addHEAD, []byte(`<script src="/MediaWarp/static/dd-danmaku/ede.js" defer></script>`+"\n")...)
	}
	if config.Web.VideoTogether { // VideoTogether
		addHEAD = append(addHEAD, []byte(`<script src="https://2gether.video/release/extension.website.user.js"></script>`+"\n")...)
	}
	htmlContent = bytes.Replace(htmlContent, []byte("</head>"), append(addHEAD, []byte("</head>")...), 1) // 将添加HEAD
	return updateBody(rw, htmlContent)
}

var _ MediaServerHandler = (*EmbyServerHandler)(nil) // 确保 EmbyServerHandler 实现 MediaServerHandler 接口

// 缓存项结构
type CacheItem struct {
	URL        string    // 缓存的URL
	ExpireTime time.Time // 过期时间
}

// 缓存管理器
var (
	redirectURLCache = make(map[string]CacheItem) // 缓存映射表，键为downloadurl+userAgent
	cacheMutex       = &sync.RWMutex{}            // 读写锁，保证并发安全
	defaultCacheTime = 30 * time.Minute           // 默认缓存时间，可通过配置修改
)
