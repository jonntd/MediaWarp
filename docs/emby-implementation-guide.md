# Emby 后端实现指南

> 基于 MediaWarp 架构的 Emby 兼容后端实现方案
> 
> 生成时间：2025-01-27

## 🎯 项目目标

实现一个与 Emby 客户端完全兼容的后端服务，支持：
- 媒体库管理和浏览
- 视频/音频流播放
- 用户认证和会话管理
- 跨平台客户端支持

## 🏗️ 架构设计

### 核心模块结构
```
emby-server/
├── internal/
│   ├── auth/           # 认证模块
│   ├── media/          # 媒体库管理
│   ├── streaming/      # 流媒体处理
│   ├── metadata/       # 元数据管理
│   ├── transcoding/    # 转码服务
│   ├── database/       # 数据库层
│   └── api/            # API 路由层
├── pkg/
│   ├── models/         # 数据模型
│   ├── utils/          # 工具函数
│   └── config/         # 配置管理
└── cmd/
    └── server/         # 主程序入口
```

## 📋 实现路线图

### Phase 1: 基础框架 (2-3 周)

#### 1.1 项目初始化
```bash
# 创建项目结构
mkdir emby-server
cd emby-server
go mod init emby-server

# 安装依赖
go get github.com/gin-gonic/gin
go get github.com/jinzhu/gorm
go get github.com/jinzhu/gorm/dialects/sqlite
go get github.com/dgrijalva/jwt-go
```

#### 1.2 基础 HTTP 服务器
```go
// cmd/server/main.go
package main

import (
    "emby-server/internal/api"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    
    // 注册 API 路由
    api.RegisterRoutes(r)
    
    r.Run(":8096") // Emby 默认端口
}
```

#### 1.3 数据库设计
```go
// pkg/models/user.go
type User struct {
    ID           string `gorm:"primary_key"`
    Name         string `gorm:"unique;not null"`
    Password     string
    IsAdmin      bool
    Configuration UserConfiguration
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// pkg/models/media.go
type MediaItem struct {
    ID           string `gorm:"primary_key"`
    Name         string
    Type         string // Movie, Episode, Audio, etc.
    Path         string
    ParentID     *string
    LibraryID    string
    MediaSources []MediaSource
    Metadata     MediaMetadata
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type MediaSource struct {
    ID           string `gorm:"primary_key"`
    MediaItemID  string
    Path         string
    Protocol     string // File, Http, etc.
    Container    string
    Size         int64
    Bitrate      int64
    MediaStreams []MediaStream
}
```

### Phase 2: 核心 API 实现 (3-4 周)

#### 2.1 认证系统
```go
// internal/auth/service.go
type AuthService struct {
    userRepo UserRepository
    jwtKey   []byte
}

func (s *AuthService) AuthenticateByName(username, password string) (*AuthResult, error) {
    user, err := s.userRepo.FindByName(username)
    if err != nil {
        return nil, err
    }
    
    if !s.verifyPassword(user.Password, password) {
        return nil, ErrInvalidCredentials
    }
    
    token, err := s.generateJWT(user)
    if err != nil {
        return nil, err
    }
    
    return &AuthResult{
        User:        user,
        AccessToken: token,
        ServerId:    s.getServerID(),
    }, nil
}
```

#### 2.2 媒体库服务
```go
// internal/media/service.go
type MediaService struct {
    mediaRepo MediaRepository
    scanner   *MediaScanner
}

func (s *MediaService) GetItems(req *GetItemsRequest) (*ItemsResponse, error) {
    items, total, err := s.mediaRepo.FindItems(req)
    if err != nil {
        return nil, err
    }
    
    return &ItemsResponse{
        Items:            items,
        TotalRecordCount: total,
        StartIndex:       req.StartIndex,
    }, nil
}

func (s *MediaService) GetPlaybackInfo(itemID, userID string, req *PlaybackInfoRequest) (*PlaybackInfoResponse, error) {
    item, err := s.mediaRepo.FindByID(itemID)
    if err != nil {
        return nil, err
    }
    
    // 构建播放信息
    mediaSources := s.buildMediaSources(item, req)
    
    return &PlaybackInfoResponse{
        MediaSources:  mediaSources,
        PlaySessionId: generatePlaySessionID(),
    }, nil
}
```

#### 2.3 流媒体服务
```go
// internal/streaming/service.go
type StreamingService struct {
    transcoder *Transcoder
}

func (s *StreamingService) StreamVideo(w http.ResponseWriter, r *http.Request, itemID string, params StreamParams) error {
    item, err := s.getMediaItem(itemID)
    if err != nil {
        return err
    }
    
    if params.Static {
        // 直接流式传输
        return s.directStream(w, r, item)
    } else {
        // 转码流式传输
        return s.transcodeStream(w, r, item, params)
    }
}
```

### Phase 3: 高级功能 (2-3 周)

#### 3.1 元数据获取
```go
// internal/metadata/provider.go
type MetadataProvider struct {
    tmdbClient *TMDBClient
    tvdbClient *TVDBClient
}

func (p *MetadataProvider) GetMovieMetadata(title string, year int) (*MovieMetadata, error) {
    // 从 TMDB 获取电影元数据
    movie, err := p.tmdbClient.SearchMovie(title, year)
    if err != nil {
        return nil, err
    }
    
    return &MovieMetadata{
        Title:       movie.Title,
        Overview:    movie.Overview,
        ReleaseDate: movie.ReleaseDate,
        Genres:      movie.Genres,
        Cast:        movie.Cast,
        Images:      movie.Images,
    }, nil
}
```

#### 3.2 转码服务
```go
// internal/transcoding/transcoder.go
type Transcoder struct {
    ffmpegPath string
}

func (t *Transcoder) TranscodeVideo(input string, output string, params TranscodeParams) error {
    args := []string{
        "-i", input,
        "-c:v", params.VideoCodec,
        "-c:a", params.AudioCodec,
        "-b:v", params.VideoBitrate,
        "-b:a", params.AudioBitrate,
        "-f", params.Container,
        output,
    }
    
    cmd := exec.Command(t.ffmpegPath, args...)
    return cmd.Run()
}
```

### Phase 4: 客户端兼容性 (2-3 周)

#### 4.1 设备配置处理
```go
// internal/api/playback.go
func (h *PlaybackHandler) GetPlaybackInfo(c *gin.Context) {
    var req PlaybackInfoRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 根据设备配置调整媒体源
    mediaSources := h.adjustMediaSourcesForDevice(req.DeviceProfile, mediaSources)
    
    response := &PlaybackInfoResponse{
        MediaSources: mediaSources,
    }
    
    c.JSON(200, response)
}
```

#### 4.2 Web 客户端支持
```go
// internal/api/web.go
func (h *WebHandler) ServeIndex(c *gin.Context) {
    // 提供定制的 index.html
    html := h.buildIndexHTML()
    c.Data(200, "text/html", html)
}

func (h *WebHandler) ServePlayerJS(c *gin.Context) {
    // 提供修改过的播放器 JavaScript
    js := h.buildPlayerJS()
    c.Data(200, "application/javascript", js)
}
```

## 🛠️ 技术栈选择

### 后端框架
- **Go + Gin**: 高性能 HTTP 服务器
- **GORM**: ORM 数据库操作
- **SQLite/PostgreSQL**: 数据存储
- **JWT**: 用户认证

### 媒体处理
- **FFmpeg**: 视频转码和处理
- **FFprobe**: 媒体信息获取

### 元数据源
- **TMDB API**: 电影数据库
- **TVDB API**: 电视剧数据库
- **MusicBrainz**: 音乐数据库

## 📊 API 实现优先级

### 🔥 高优先级 (MVP)
1. **认证 API**
   - `POST /Users/AuthenticateByName`
   - `GET /Users/Me`

2. **媒体库 API**
   - `GET /Items`
   - `GET /Users/{UserId}/Items`
   - `POST /Items/{Id}/PlaybackInfo`

3. **流媒体 API**
   - `GET /Videos/{Id}/stream`
   - `GET /Audio/{Id}/stream`

4. **系统 API**
   - `GET /System/Info`
   - `GET /System/Info/Public`

### 🟡 中优先级
1. **用户界面 API**
   - `GET /web/index.html`
   - `GET /Users/{UserId}/Views`

2. **播放状态 API**
   - `POST /Sessions/Playing`
   - `POST /Sessions/Playing/Progress`
   - `POST /Sessions/Playing/Stopped`

3. **图片 API**
   - `GET /Items/{Id}/Images/{Type}`

### 🟢 低优先级
1. **高级功能**
   - 直播电视
   - 插件系统
   - 同步功能

## 🧪 测试策略

### 单元测试
```go
// internal/media/service_test.go
func TestMediaService_GetItems(t *testing.T) {
    service := NewMediaService(mockRepo)
    
    req := &GetItemsRequest{
        Limit: 10,
        StartIndex: 0,
    }
    
    response, err := service.GetItems(req)
    
    assert.NoError(t, err)
    assert.NotNil(t, response)
    assert.Len(t, response.Items, 10)
}
```

### 集成测试
```go
// test/integration/api_test.go
func TestAuthenticationAPI(t *testing.T) {
    server := setupTestServer()
    defer server.Close()
    
    // 测试用户认证
    resp := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/Users/AuthenticateByName", strings.NewReader(`{
        "Username": "testuser",
        "Pw": "password"
    }`))
    
    server.ServeHTTP(resp, req)
    
    assert.Equal(t, 200, resp.Code)
}
```

### 客户端兼容性测试
- Emby Web 客户端
- Emby Android 客户端
- Emby iOS 客户端
- 第三方客户端 (Infuse, VLC 等)

## 📈 性能优化

### 数据库优化
```sql
-- 为常用查询添加索引
CREATE INDEX idx_media_items_parent_id ON media_items(parent_id);
CREATE INDEX idx_media_items_library_id ON media_items(library_id);
CREATE INDEX idx_media_items_type ON media_items(type);
```

### 缓存策略
```go
// internal/cache/service.go
type CacheService struct {
    redis *redis.Client
}

func (s *CacheService) CachePlaybackInfo(itemID string, info *PlaybackInfoResponse) error {
    data, _ := json.Marshal(info)
    return s.redis.Set(fmt.Sprintf("playback:%s", itemID), data, 5*time.Minute).Err()
}
```

### 流媒体优化
- HTTP Range 请求支持
- 分段传输编码
- 客户端缓存控制

## 🚀 部署方案

### Docker 部署
```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o emby-server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ffmpeg
WORKDIR /root/
COPY --from=builder /app/emby-server .
EXPOSE 8096
CMD ["./emby-server"]
```

### 配置文件
```yaml
# config.yaml
server:
  port: 8096
  host: "0.0.0.0"

database:
  type: "sqlite"
  path: "/data/emby.db"

media:
  libraries:
    - name: "Movies"
      path: "/media/movies"
      type: "movies"
    - name: "TV Shows"
      path: "/media/tv"
      type: "tvshows"

transcoding:
  ffmpeg_path: "/usr/bin/ffmpeg"
  temp_path: "/tmp/transcoding"
```

---

## 📚 参考资源

- [Emby 官方 API 文档](https://dev.emby.media/)
- [MediaWarp 项目](https://github.com/your-repo/MediaWarp)
- [FFmpeg 文档](https://ffmpeg.org/documentation.html)
- [TMDB API 文档](https://developers.themoviedb.org/3)

---

*文档更新时间: 2025-01-27*
*基于 MediaWarp 架构和 Emby API 规范设计*
