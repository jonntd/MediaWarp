package handler

import (
	"MediaWarp/constants"
	"MediaWarp/internal/config"
	"MediaWarp/internal/logging"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

// 将需要修改上游响应的处理器包装成一个 gin.HandlerFunc 处理器
func responseModifyCreater(proxy *httputil.ReverseProxy, modifyResponseFN func(rw *http.Response) error) gin.HandlerFunc {
	proxy.ModifyResponse = modifyResponseFN
	return func(ctx *gin.Context) {
		proxy.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

// 根据 Strm 文件路径识别 Strm 文件类型
// 返回 Strm 文件类型和一个可选配置
func recgonizeStrmFileType(strmFilePath string) (constants.StrmFileType, any, any) {
	logging.Debug("识别 Strm 文件类型，路径：" + strmFilePath)

	// 1. MediaSync 检查 - 检查是否匹配任何 MediaSync 服务器的本地路径
	for _, server := range config.MediaSync {
		if strings.HasPrefix(strmFilePath, server.LocalPath) {
			logging.Debug(strmFilePath + " 匹配 MediaSync 服务器：" + server.Name + "，路径：" + server.LocalPath + "，类型：HTTPStrm")
			return constants.HTTPStrm, nil, nil
		}
	}

	// 2. 读取 .strm 文件内容来判断类型
	if strings.HasSuffix(strings.ToLower(strmFilePath), ".strm") {
		content, err := os.ReadFile(strmFilePath)
		if err != nil {
			logging.Warning("读取 strm 文件失败：" + strmFilePath + "，错误：" + err.Error())
			// 读取失败时，默认认为是 HTTPStrm，让后续流程处理
			return constants.HTTPStrm, nil, nil
		}

		contentStr := strings.TrimSpace(string(content))
		logging.Debug("Strm 文件内容：" + contentStr)

		// 3. 根据内容判断类型
		if contentStr != "" {
			// 如果包含 rclone 协议格式（如 115://, alist://, onedrive:// 等）
			if strings.Contains(contentStr, "://") && !strings.HasPrefix(contentStr, "http://") && !strings.HasPrefix(contentStr, "https://") {
				logging.Debug(strmFilePath + " 检测到 rclone 格式内容：" + contentStr + "，类型：HTTPStrm")
				return constants.HTTPStrm, nil, nil
			}

			// 如果是标准 HTTP/HTTPS 链接
			if strings.HasPrefix(contentStr, "http://") || strings.HasPrefix(contentStr, "https://") {
				logging.Debug(strmFilePath + " 检测到 HTTP 链接：" + contentStr + "，类型：HTTPStrm")
				return constants.HTTPStrm, nil, nil
			}
		}
	}

	// 4. 如果都不匹配，但是是 .strm 文件，仍然标记为 HTTPStrm
	// 这样可以确保所有 .strm 文件都会被处理，避免 UnknownStrm 导致的问题
	logging.Debug(strmFilePath + " 未匹配具体类型，但是 .strm 文件，默认标记为 HTTPStrm")
	return constants.HTTPStrm, nil, nil
}

// 读取响应体
//
// 读取响应体，解压缩 GZIP、Brotli 数据（若响应体被压缩）
func readBody(rw *http.Response) ([]byte, error) {
	encoding := rw.Header.Get("Content-Encoding")

	var reader io.Reader
	switch encoding {
	case "gzip":
		logging.Debug("解码 GZIP 数据")
		gr, err := gzip.NewReader(rw.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader error: %w", err)
		}
		defer gr.Close()
		reader = gr

	case "br":
		logging.Debug("解码 Brotli 数据")
		reader = brotli.NewReader(rw.Body)

	case "": // 无压缩
		logging.Debug("无压缩数据")
		reader = rw.Body

	default:
		return nil, fmt.Errorf("unsupported Content-Encoding: %s", encoding)
	}
	return io.ReadAll(reader)
}

// 更新响应体
//
// 修改响应体、更新Content-Length
func updateBody(rw *http.Response, content []byte) error {
	encoding := rw.Header.Get("Content-Encoding")
	var (
		compressed bytes.Buffer
		writer     io.Writer
	)

	// 根据原始编码选择压缩方式
	switch encoding {
	case "gzip":
		logging.Debug("使用 GZIP 重新编码数据")
		gw := gzip.NewWriter(&compressed)
		defer gw.Close()
		writer = gw

	case "br":
		logging.Debug("使用 Brotli 重新编码数据")
		bw := brotli.NewWriter(&compressed)
		defer bw.Close()
		writer = bw

	case "": // 无压缩
		logging.Debug("无压缩数据")
		writer = &compressed

	default:
		logging.Warningf("不支持的重新编码：%s，将不对数据进行压缩编码", encoding)
		rw.Header.Del("Content-Encoding")
	}

	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("compression write error: %w", err)
	}

	// Brotli 需要显式 Flush
	if bw, ok := writer.(*brotli.Writer); ok {
		if err := bw.Flush(); err != nil {
			return err
		}
	}

	// 设置新 Body
	rw.Body = io.NopCloser(bytes.NewReader(compressed.Bytes()))
	rw.ContentLength = int64(compressed.Len())
	rw.Header.Set("Content-Length", strconv.Itoa(compressed.Len())) // 更新响应头

	return nil
}
