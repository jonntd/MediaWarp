package _115

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	UserAgent            = "Mozilla/5.0 115Browser/23.9.3.2"
	APIURLGetFiles       = "https://webapi.115.com/files"
	APIURLGetDownloadURL = "https://proapi.115.com/app/chrome/downurl"
	APIURLGetDirID       = "https://webapi.115.com/files/getid"
	APIURLDeleteFile     = "https://webapi.115.com/rb/delete"
	APIURLAddDir         = "https://webapi.115.com/files/add"
	APIURLMoveFile       = "https://webapi.115.com/files/move"
	APIURLRenameFile     = "https://webapi.115.com/files/batch_rename"
	APIURLLoginCheck     = "https://passportapi.115.com/app/1.0/web/1.0/check/sso"
)

var (
	ErrNotFound = errors.New("not found")
)

func APIGetFiles(client *resty.Client, cid string, pageSize int64, offset int64) (*APIGetFilesResp, error) {
	result := APIGetFilesResp{}
	_, err := client.R().
		SetQueryParams(map[string]string{
			"aid":              "1",
			"cid":              cid,
			"o":                "user_ptime",
			"asc":              "0",
			"offset":           strconv.FormatInt(offset, 10),
			"show_dir":         "1",
			"limit":            strconv.FormatInt(pageSize, 10),
			"snap":             "0",
			"record_open_time": "1",
			"format":           "json",
			"fc_mix":           "0",
		}).
		SetResult(&result).
		ForceContentType("application/json").
		Get(APIURLGetFiles)
	if err != nil {
		return nil, fmt.Errorf("api get files fail, err: %v", err)
	}

	return &result, nil
}

func APIGetDownloadURL(client *resty.Client, pickCode string, userAgent string) (*DownloadInfo, error) {
	key := GenerateKey()
	params, _ := json.Marshal(map[string]string{"pickcode": pickCode})

	result := APIBaseResp{}
	_, err := client.R().
		SetQueryParam("t", strconv.FormatInt(time.Now().Unix(), 10)).
		SetFormData(map[string]string{
			"data": string(Encode(params, key)),
		}).
		SetResult(&result).
		ForceContentType("application/json").
		SetHeader("User-Agent", userAgent).
		Post(APIURLGetDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("api get download url fail, err: %v", err)
	}

	var encodedData string
	if err = json.Unmarshal(result.Data, &encodedData); err != nil {
		return nil, fmt.Errorf("api get download url, call json.Unmarshal fail, body: %s", string(result.Data))
	}
	decodedData, err := Decode(encodedData, key)
	if err != nil {
		return nil, fmt.Errorf("api get download url, call Decode fail, err: %w", err)
	}

	resp := DownloadData{}
	if err := json.Unmarshal(decodedData, &resp); err != nil {
		return nil, fmt.Errorf("api get download url, call json.Unmarshal fail, body: %s", string(decodedData))
	}

	for _, info := range resp {
		fileSize, _ := info.FileSize.Int64()
		if fileSize == 0 {
			return nil, ErrNotFound
		}
		return info, nil
	}

	return nil, nil
}

func APIGetDirID(client *resty.Client, dir string) (*APIGetDirIDResp, error) {
	if strings.HasPrefix(dir, "/") {
		dir = dir[1:]
	}

	result := APIGetDirIDResp{}
	_, err := client.R().
		SetQueryParam("path", dir).
		SetResult(&result).
		ForceContentType("application/json").
		Get(APIURLGetDirID)
	if err != nil {
		return nil, fmt.Errorf("api get dir id fail, err: %v", err)
	}

	return &result, nil
}

func APILoginCheck(client *resty.Client) (int64, error) {
	result := APILoginCheckResp{}
	_, err := client.R().
		SetResult(&result).
		ForceContentType("application/json").
		Get(APIURLLoginCheck)
	if err != nil {
		return 0, fmt.Errorf("api login check fail, err: %v", err)
	}

	userID, _ := result.Data.UserID.Int64()
	return userID, nil
}

type DriveClient struct {
	HttpClient   *resty.Client
	cache        gcache.Cache
	reserveProxy *httputil.ReverseProxy
	limiter      *rate.Limiter
}
type File interface {
	GetName() string
	GetSize() int64
	GetUpdateTime() time.Time
	GetCreateTime() time.Time
	IsDir() bool
}

// type DriveClient interface {
// 	GetFiles(dir string) ([]File, error)
// 	GetFile(filePath string) (File, error)
// 	RemoveFile(filePath string) error
// 	MoveFile(srcPath string, dstPath string) error
// 	MakeDir(dir string) error
// 	ServeContent(w http.ResponseWriter, req *http.Request, fi File)
// }

func MustNew115DriveClient(cookie string) *DriveClient {
	// httpClient := resty.New().SetCookie(&http.Cookie{
	// 	Name:     "UID",
	// 	Value:    uid,
	// 	Domain:   "www.115.com",
	// 	Path:     "/",
	// 	HttpOnly: true,
	// }).SetCookie(&http.Cookie{
	// 	Name:     "CID",
	// 	Value:    cid,
	// 	Domain:   "www.115.com",
	// 	Path:     "/",
	// 	HttpOnly: true,
	// }).SetCookie(&http.Cookie{
	// 	Name:     "SEID",
	// 	Value:    seid,
	// 	Domain:   "www.115.com",
	// 	Path:     "/",
	// 	HttpOnly: true,
	// }).SetHeader("User-Agent", UserAgent)

	cookies := strings.Split(cookie, "; ")

	// Create a new Resty client
	httpClient := resty.New()

	// Iterate over the cookies and set them in the client
	for _, cookie := range cookies {
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) != 2 {
			continue
		}

		httpClient.SetCookie(&http.Cookie{
			Name:     parts[0],
			Value:    parts[1],
			Domain:   "www.115.com",
			Path:     "/",
			HttpOnly: true,
		})
	}

	httpClient.SetHeader("User-Agent", UserAgent)
	client := &DriveClient{
		HttpClient: httpClient,
		cache:      gcache.New(10000).LFU().Build(),
		limiter:    rate.NewLimiter(5, 1),
		reserveProxy: &httputil.ReverseProxy{
			Transport: httpClient.GetClient().Transport,
			Director: func(req *http.Request) {
				req.Header.Set("Referer", "https://115.com/")
				req.Header.Set("User-Agent", UserAgent)
				req.Header.Set("Host", req.Host)
			},
		},
	}

	// login check
	userID, err := APILoginCheck(client.HttpClient)
	if err != nil || userID <= 0 {
		logrus.WithError(err).Panicf("115 drive login fail")
	}
	fmt.Printf("\n【115 drive login succ, user_id: %d 】\n", userID)

	return client
}

func (c *DriveClient) GetFiles(dir string) ([]File, error) {
	dir = slashClean(dir)
	cacheKey := fmt.Sprintf("files:%s", dir)
	if value, err := c.cache.Get(cacheKey); err == nil {
		return value.([]File), nil
	}

	c.limiter.Wait(context.Background())
	getDirIDResp, err := APIGetDirID(c.HttpClient, dir)
	if err != nil {
		return nil, err
	}
	cid := getDirIDResp.CategoryID.String()

	pageSize := int64(1000)
	offset := int64(0)
	files := make([]File, 0)
	for {
		resp, err := APIGetFiles(c.HttpClient, cid, pageSize, offset)
		if err != nil {
			return nil, err
		}

		for idx := range resp.Data {
			files = append(files, &resp.Data[idx])
		}

		offset = resp.Offset + pageSize
		if offset >= resp.Count {
			break
		}
	}
	if err := c.cache.SetWithExpire(cacheKey, files, time.Minute*2); err != nil {
		logrus.WithError(err).Errorf("call c.cache.SetWithExpire fail, dir: %s", dir)
	}

	return files, nil
}

func (c *DriveClient) GetFile(filePath string) (File, error) {
	filePath = slashClean(filePath)
	if filePath == "/" || len(filePath) == 0 {
		return &FileInfo{CategoryID: "0"}, nil
	}

	filePath = strings.TrimRight(filePath, "/")
	dir, fileName := path.Split(filePath)

	files, err := c.GetFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.GetName() == fileName {
			return file, nil
		}
	}

	return nil, ErrNotFound
}

func (c *DriveClient) GetFileURL(file File, userAgent string) (string, error) {
	pickCode := file.(*FileInfo).PickCode
	cacheKey := fmt.Sprintf("url:%s:%s", pickCode, userAgent)
	if value, err := c.cache.Get(cacheKey); err == nil {
		logrus.Infoln("使用缓存")
		return value.(string), nil
	}

	c.limiter.Wait(context.Background())
	info, err := APIGetDownloadURL(c.HttpClient, pickCode, userAgent)
	if err != nil {
		return "", err
	}

	if err := c.cache.SetWithExpire(cacheKey, info.URL.URL, time.Minute*2); err != nil {
		logrus.WithError(err).Errorf("call c.cache.SetWithExpire fail, url: %s", info.URL.URL)
	}

	return info.URL.URL, nil
}

func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}
