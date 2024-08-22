package controllers

import (
	_115 "MediaWarp/115"
	"MediaWarp/schemas/emby"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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
func StreamHandler(ctx *gin.Context, DriveClient *_115.DriveClient) {
	mediaSourceID := ctx.Query("mediasourceid")
	itemInfoUri := config.Origin + "/Items/?Ids=" + mediaSourceID + "&Fields=Path,MediaSources&Limit=1&api_key=" + config.ApiKey
	// fmt.Println("itemInfoUri ", itemInfoUri)
	resp, err := http.Get(itemInfoUri)
	if err != nil {
		logger.ServerLogger.Warning("请求失败：", err)
		return
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.ServerLogger.Warning("读取响应体失败：", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应体失败"})
		return
	}

	var UserItemsResponse emby.UserItemsResponse
	err = json.Unmarshal(body, &UserItemsResponse)
	if err != nil {
		logger.ServerLogger.Warning("解析Json错误：", err)
		return
	}
	if len(UserItemsResponse.Items) > 0 {
		for _, mediasource := range UserItemsResponse.Items[0].MediaSources {
			if *mediasource.ID == mediaSourceID {
				logger.ServerLogger.Info("302重定向：", *mediasource.Path)
				desiredPath := strings.Replace(*mediasource.Path, config.MountPath, "", 1)
				desiredPath = ensureLeadingSlash(desiredPath)
				// logger.ServerLogger.Info("desiredPath：", desiredPath)
				files, err := DriveClient.GetFile(desiredPath)
				if err != nil {
					DefaultHandler(ctx)
					return
				}
				userAgent := ctx.Request.Header.Get("User-Agent")
				// logger.ServerLogger.Info("userAgent", userAgent)
				down_url, err := DriveClient.GetFileURL(files, userAgent)
				if err != nil {
					DefaultHandler(ctx)
					return
				}
				ctx.Redirect(302, down_url)
				return
			}
		}
	}
}
