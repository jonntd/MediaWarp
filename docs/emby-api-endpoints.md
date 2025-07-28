# Emby Server API 端点详细文档

> 基于官方文档和实际使用整理的详细端点信息
> 
> 生成时间：2025-01-27

## 🔐 认证端点

### 用户认证
```http
POST /Users/AuthenticateByName
Content-Type: application/json

{
  "Username": "string",
  "Pw": "string",
  "Password": "string"
}
```

### 获取当前用户信息
```http
GET /Users/Me
Authorization: MediaBrowser Token="your_token"
```

## 📚 媒体库端点

### ItemsService - 核心媒体项目 API

#### 获取媒体项目列表
```http
GET /Items
Parameters:
- Ids: string (逗号分隔的ID列表)
- UserId: string (用户ID)
- Limit: integer (返回数量限制)
- StartIndex: integer (起始索引)
- Fields: string (返回字段，逗号分隔)
- Recursive: boolean (是否递归搜索)
- IncludeItemTypes: string (包含的项目类型)
- ExcludeItemTypes: string (排除的项目类型)
- SortBy: string (排序字段)
- SortOrder: string (排序方向: Ascending/Descending)
- ParentId: string (父项目ID)
```

#### 获取用户媒体项目
```http
GET /Users/{UserId}/Items
Parameters:
- UserId: string (路径参数)
- ParentId: string (父项目ID)
- Limit: integer
- StartIndex: integer
- Fields: string
- Recursive: boolean
- IncludeItemTypes: string
- SortBy: string
- SortOrder: string
```

#### 获取继续播放项目
```http
GET /Users/{UserId}/Items/Resume
Parameters:
- UserId: string (路径参数)
- Limit: integer
- Fields: string
- MediaTypes: string
```

#### 获取单个项目详情
```http
GET /Items/{Id}
Parameters:
- Id: string (路径参数)
- UserId: string
- Fields: string
```

## 🎵 媒体流端点

### 视频流
```http
GET /Videos/{Id}/stream
Parameters:
- Id: string (视频ID)
- MediaSourceId: string (媒体源ID)
- Static: boolean (是否静态流)
- DeviceId: string (设备ID)
- api_key: string (API密钥)
- Container: string (容器格式)
- AudioCodec: string (音频编解码器)
- VideoCodec: string (视频编解码器)
- MaxAudioBitrate: integer (最大音频比特率)
- MaxVideoBitrate: integer (最大视频比特率)
- Width: integer (视频宽度)
- Height: integer (视频高度)
```

### 音频流
```http
GET /Audio/{Id}/stream
Parameters:
- Id: string (音频ID)
- MediaSourceId: string
- Static: boolean
- DeviceId: string
- api_key: string
- Container: string
- AudioCodec: string
- MaxAudioBitrate: integer
```

### 获取播放信息
```http
POST /Items/{Id}/PlaybackInfo
Content-Type: application/json
Parameters:
- Id: string (路径参数)
- UserId: string

Body:
{
  "UserId": "string",
  "MaxStreamingBitrate": integer,
  "StartTimeTicks": integer,
  "AudioStreamIndex": integer,
  "SubtitleStreamIndex": integer,
  "MaxAudioChannels": integer,
  "MediaSourceId": "string",
  "LiveStreamId": "string",
  "DeviceProfile": {
    // 设备配置信息
  }
}
```

## 🖼️ 图片端点

### 获取项目图片
```http
GET /Items/{Id}/Images/{Type}
Parameters:
- Id: string (项目ID)
- Type: string (图片类型: Primary, Backdrop, Logo, etc.)
- MaxWidth: integer (最大宽度)
- MaxHeight: integer (最大高度)
- Quality: integer (质量 1-100)
- Tag: string (图片标签)
```

### 获取用户头像
```http
GET /Users/{Id}/Images/{Type}
Parameters:
- Id: string (用户ID)
- Type: string (图片类型)
- MaxWidth: integer
- MaxHeight: integer
- Quality: integer
```

## 📺 用户界面端点

### 获取 Web 首页
```http
GET /web/index.html
```

### 获取播放器 JavaScript
```http
GET /web/modules/htmlvideoplayer/basehtmlplayer.js
Parameters:
- v: string (版本号)
```

### 获取用户视图
```http
GET /Users/{UserId}/Views
Parameters:
- UserId: string (路径参数)
- IncludeExternalContent: boolean
- PresetViews: string
```

## 🎮 播放状态端点

### 报告播放开始
```http
POST /Sessions/Playing
Content-Type: application/json

{
  "ItemId": "string",
  "MediaSourceId": "string",
  "PositionTicks": integer,
  "PlayMethod": "string",
  "PlaySessionId": "string"
}
```

### 报告播放进度
```http
POST /Sessions/Playing/Progress
Content-Type: application/json

{
  "ItemId": "string",
  "MediaSourceId": "string",
  "PositionTicks": integer,
  "PlayMethod": "string",
  "PlaySessionId": "string",
  "IsPaused": boolean,
  "IsMuted": boolean,
  "VolumeLevel": integer
}
```

### 报告播放停止
```http
POST /Sessions/Playing/Stopped
Content-Type: application/json

{
  "ItemId": "string",
  "MediaSourceId": "string",
  "PositionTicks": integer,
  "PlaySessionId": "string"
}
```

## 🔧 系统端点

### 获取系统信息
```http
GET /System/Info
```

### 获取系统配置
```http
GET /System/Configuration
```

### 获取公共系统信息
```http
GET /System/Info/Public
```

## 👥 用户管理端点

### 获取公共用户列表
```http
GET /Users/Public
```

### 获取用户配置
```http
GET /Users/{UserId}/Configuration
Parameters:
- UserId: string (路径参数)
```

### 更新用户配置
```http
POST /Users/{UserId}/Configuration
Content-Type: application/json
Parameters:
- UserId: string (路径参数)

Body: UserConfiguration object
```

## 📱 会话管理端点

### 获取活动会话
```http
GET /Sessions
Parameters:
- ControllableByUserId: string
- DeviceId: string
- ActiveWithinSeconds: integer
```

### 发送消息到会话
```http
POST /Sessions/{SessionId}/Message
Content-Type: application/json
Parameters:
- SessionId: string (路径参数)

Body:
{
  "Header": "string",
  "Text": "string",
  "TimeoutMs": integer
}
```

## 🔍 搜索端点

### 搜索媒体项目
```http
GET /Search/Hints
Parameters:
- SearchTerm: string (搜索词)
- UserId: string
- Limit: integer
- IncludeItemTypes: string
- IncludeStudios: boolean
- IncludeGenres: boolean
- IncludePeople: boolean
- IncludeMedia: boolean
- IncludeArtists: boolean
```

## 📋 播放列表端点

### 创建播放列表
```http
POST /Playlists
Content-Type: application/json

{
  "Name": "string",
  "UserId": "string",
  "ItemIdList": ["string"],
  "MediaType": "string"
}
```

### 添加项目到播放列表
```http
POST /Playlists/{Id}/Items
Parameters:
- Id: string (播放列表ID)
- UserId: string
- Ids: string (逗号分隔的项目ID)
```

## 🏷️ 元数据端点

### 获取类型列表
```http
GET /Genres
Parameters:
- UserId: string
- StartIndex: integer
- Limit: integer
- Fields: string
- ParentId: string
- IncludeItemTypes: string
```

### 获取人物信息
```http
GET /Persons/{Name}
Parameters:
- Name: string (路径参数)
- UserId: string
```

## 📊 统计端点

### 获取播放统计
```http
GET /Users/{UserId}/PlayingItems
Parameters:
- UserId: string (路径参数)
- Limit: integer
- Fields: string
```

### 获取最新添加项目
```http
GET /Users/{UserId}/Items/Latest
Parameters:
- UserId: string (路径参数)
- Limit: integer
- Fields: string
- IncludeItemTypes: string
- ParentId: string
- GroupItems: boolean
```

---

## 📝 请求/响应格式

### 通用请求头
```http
Content-Type: application/json
Authorization: MediaBrowser Token="your_api_key"
X-Emby-Authorization: MediaBrowser UserId="user_id", Client="client_name", Device="device_name", DeviceId="device_id", Version="version", Token="token"
```

### 通用响应格式
```json
{
  "Items": [],
  "TotalRecordCount": 0,
  "StartIndex": 0
}
```

### 错误响应格式
```json
{
  "ErrorCode": "string",
  "ErrorMessage": "string"
}
```

---

## 🎯 MediaWarp 当前实现对比

### ✅ 已实现的端点
- `GET /Items` (ItemsService.ItemsServiceQueryItem)
- `GET /web/index.html` (EmbyServer.GetIndexHtml)
- `POST /Items/{Id}/PlaybackInfo` (ModifyPlaybackInfo)
- `GET /Videos/{Id}/stream` (VideosHandler)
- `GET /web/modules/htmlvideoplayer/basehtmlplayer.js` (ModifyBaseHtmlPlayer)

### ❌ 未实现的核心端点
- 用户认证相关端点
- 系统信息端点
- 会话管理端点
- 播放状态报告端点
- 搜索端点
- 用户管理端点
- 图片服务端点

---

*文档更新时间: 2025-01-27*
*基于 Emby Server 官方 API 文档和 MediaWarp 实现整理*
