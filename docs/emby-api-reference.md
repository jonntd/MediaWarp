# Emby Server REST API 完整参考文档

> 基于官方文档整理：https://dev.emby.media/reference/RestAPI.html
> 
> 生成时间：2025-01-27

## 📖 概述

Emby Server API 可以通过以下方式访问：
```
http[s]://hostname:port/emby/{apipath}
```

## 🔐 认证方式

### 1. 用户认证 (User Authentication)
通过用户名和密码登录，适用于客户端应用开发。

### 2. API Key 认证 (API Key Authentication)
使用静态令牌访问，适用于服务器集成场景。

## 📋 完整 API 服务列表

### 🎯 核心服务 (Core Services)

#### ActivityLogService
- **功能**: 活动日志管理
- **文档**: [ActivityLogService](https://dev.emby.media/reference/RestAPI/ActivityLogService.html)

#### ItemsService ⭐
- **功能**: 获取媒体项目信息
- **文档**: [Item Information](https://dev.emby.media/doc/restapi/Item-Information.html)
- **主要端点**:
  - `GET /Items` - 基于查询获取项目
  - `GET /Users/{UserId}/Items` - 获取用户项目
  - `GET /Users/{UserId}/Items/Resume` - 获取继续播放项目

#### UserService ⭐
- **功能**: 用户管理
- **文档**: [UserService](https://dev.emby.media/reference/RestAPI/UserService.html)

#### SessionsService ⭐
- **功能**: 会话管理和远程控制
- **文档**: [Remote control](https://dev.emby.media/doc/restapi/Remote-Control.html)

#### SystemService ⭐
- **功能**: 系统信息和配置
- **文档**: [SystemService](https://dev.emby.media/reference/RestAPI/SystemService.html)

### 🎵 媒体处理服务 (Media Services)

#### AudioService ⭐
- **功能**: 音频流处理
- **文档**: [Audio streaming](https://dev.emby.media/doc/restapi/Audio-Streaming.html)

#### VideoService ⭐
- **功能**: 视频流处理
- **文档**: [Video streaming](https://dev.emby.media/doc/restapi/Video-Streaming.html)

#### VideosService
- **功能**: 视频管理操作
- **文档**: [VideosService](https://dev.emby.media/reference/RestAPI/VideosService.html)
- **主要端点**:
  - `DELETE /Videos/{Id}/AlternateSources` - 删除备用视频源
  - `POST /Videos/MergeVersions` - 合并视频版本

#### UniversalAudioService
- **功能**: 通用音频服务
- **文档**: [Audio streaming](https://dev.emby.media/doc/restapi/Audio-Streaming.html)

#### ImageService ⭐
- **功能**: 图片处理和缩略图
- **文档**: [Images](https://dev.emby.media/doc/restapi/Images.html)

#### SubtitleService
- **功能**: 字幕处理
- **文档**: [SubtitleService](https://dev.emby.media/reference/RestAPI/SubtitleService.html)

### 🌊 流媒体服务 (Streaming Services)

#### DynamicHlsService ⭐
- **功能**: HTTP Live Streaming 动态处理
- **文档**: [Http Live Streaming](https://dev.emby.media/doc/restapi/Http-Live-Streaming.html)

#### VideoHlsService
- **功能**: 视频 HLS 服务
- **文档**: [VideoHlsService](https://dev.emby.media/reference/RestAPI/VideoHlsService.html)

#### HlsSegmentService
- **功能**: HLS 分段服务
- **文档**: [HlsSegmentService](https://dev.emby.media/reference/RestAPI/HlsSegmentService.html)

#### LiveStreamService
- **功能**: 直播流服务
- **文档**: [LiveStreamService](https://dev.emby.media/reference/RestAPI/LiveStreamService.html)

### 📚 内容管理服务 (Content Management)

#### LibraryService ⭐
- **功能**: 媒体库浏览和管理
- **文档**: [Browsing the Library](https://dev.emby.media/doc/restapi/Browsing-the-Library.html)

#### UserLibraryService ⭐
- **功能**: 用户媒体库管理
- **文档**: [Browsing the Library](https://dev.emby.media/doc/restapi/Browsing-the-Library.html)

#### LibraryStructureService
- **功能**: 媒体库结构管理
- **文档**: [LibraryStructureService](https://dev.emby.media/reference/RestAPI/LibraryStructureService.html)

#### CollectionService
- **功能**: 收藏集管理
- **文档**: [CollectionService](https://dev.emby.media/reference/RestAPI/CollectionService.html)

#### PlaylistService ⭐
- **功能**: 播放列表管理
- **文档**: [Playlists](https://dev.emby.media/doc/restapi/Playlists.html)

#### ItemUpdateService
- **功能**: 媒体项目更新
- **文档**: [ItemUpdateService](https://dev.emby.media/reference/RestAPI/ItemUpdateService.html)

#### ItemRefreshService
- **功能**: 媒体项目刷新
- **文档**: [ItemRefreshService](https://dev.emby.media/reference/RestAPI/ItemRefreshService.html)

#### ItemLookupService
- **功能**: 媒体项目查找
- **文档**: [ItemLookupService](https://dev.emby.media/reference/RestAPI/ItemLookupService.html)

### 🎮 播放控制服务 (Playback Services)

#### PlaystateService ⭐
- **功能**: 播放状态管理
- **文档**: [PlaystateService](https://dev.emby.media/reference/RestAPI/PlaystateService.html)

#### MediaInfoService
- **功能**: 媒体信息获取
- **文档**: [MediaInfoService](https://dev.emby.media/reference/RestAPI/MediaInfoService.html)

### 🖥️ 用户界面服务 (UI Services)

#### WebAppService
- **功能**: Web 应用服务
- **文档**: [WebAppService](https://dev.emby.media/reference/RestAPI/WebAppService.html)

#### DisplayPreferencesService
- **功能**: 显示偏好设置
- **文档**: [DisplayPreferencesService](https://dev.emby.media/reference/RestAPI/DisplayPreferencesService.html)

#### UserViewsService
- **功能**: 用户视图管理
- **文档**: [Browsing the Library](https://dev.emby.media/doc/restapi/Browsing-the-Library.html)

#### BrandingService
- **功能**: 品牌定制
- **文档**: [BrandingService](https://dev.emby.media/reference/RestAPI/BrandingService.html)

### 📺 直播电视服务 (Live TV Services)

#### LiveTvService
- **功能**: 直播电视管理
- **文档**: [LiveTvService](https://dev.emby.media/reference/RestAPI/LiveTvService.html)

#### ChannelService
- **功能**: 频道管理
- **文档**: [ChannelService](https://dev.emby.media/reference/RestAPI/ChannelService.html)

### 🔧 系统管理服务 (System Management)

#### ConfigurationService ⭐
- **功能**: 系统配置管理
- **文档**: [ConfigurationService](https://dev.emby.media/reference/RestAPI/ConfigurationService.html)

#### DeviceService
- **功能**: 设备管理
- **文档**: [DeviceService](https://dev.emby.media/reference/RestAPI/DeviceService.html)

#### NotificationsService
- **功能**: 通知服务
- **文档**: [NotificationsService](https://dev.emby.media/reference/RestAPI/NotificationsService.html)

#### UserNotificationsService
- **功能**: 用户通知服务
- **文档**: [UserNotificationsService](https://dev.emby.media/reference/RestAPI/UserNotificationsService.html)

#### ScheduledTaskService
- **功能**: 计划任务管理
- **文档**: [ScheduledTaskService](https://dev.emby.media/reference/RestAPI/ScheduledTaskService.html)

#### PluginService
- **功能**: 插件管理
- **文档**: [PluginService](https://dev.emby.media/reference/RestAPI/PluginService.html)

#### PackageService
- **功能**: 包管理
- **文档**: [PackageService](https://dev.emby.media/reference/RestAPI/PackageService.html)

### 🔍 搜索和发现服务 (Search & Discovery)

#### ArtistsService
- **功能**: 艺术家管理
- **文档**: [ArtistsService](https://dev.emby.media/reference/RestAPI/ArtistsService.html)

#### PersonsService
- **功能**: 人物信息管理
- **文档**: [PersonsService](https://dev.emby.media/reference/RestAPI/PersonsService.html)

#### GenresService
- **功能**: 类型管理
- **文档**: [GenresService](https://dev.emby.media/reference/RestAPI/GenresService.html)

#### MusicGenresService
- **功能**: 音乐类型管理
- **文档**: [MusicGenresService](https://dev.emby.media/reference/RestAPI/MusicGenresService.html)

#### StudiosService
- **功能**: 工作室管理
- **文档**: [StudiosService](https://dev.emby.media/reference/RestAPI/StudiosService.html)

#### TagService
- **功能**: 标签管理
- **文档**: [TagService](https://dev.emby.media/reference/RestAPI/TagService.html)

#### SuggestionsService
- **功能**: 推荐服务
- **文档**: [SuggestionsService](https://dev.emby.media/reference/RestAPI/SuggestionsService.html)

#### InstantMixService
- **功能**: 即时混音
- **文档**: [InstantMixService](https://dev.emby.media/reference/RestAPI/InstantMixService.html)

### 🎬 专门内容服务 (Specialized Content)

#### MoviesService
- **功能**: 电影专门服务
- **文档**: [MoviesService](https://dev.emby.media/reference/RestAPI/MoviesService.html)

#### TvShowsService
- **功能**: 电视剧专门服务
- **文档**: [TvShowsService](https://dev.emby.media/reference/RestAPI/TvShowsService.html)

#### TrailersService
- **功能**: 预告片服务
- **文档**: [TrailersService](https://dev.emby.media/reference/RestAPI/TrailersService.html)

#### GameGenresService
- **功能**: 游戏类型服务
- **文档**: [GameGenresService](https://dev.emby.media/reference/RestAPI/GameGenresService.html)

### 🔄 同步和备份服务 (Sync & Backup)

#### SyncService
- **功能**: 同步服务
- **文档**: [Sync](https://dev.emby.media/doc/restapi/Sync.html)

#### BackupApi
- **功能**: 备份 API
- **文档**: [REST API Documentation](https://dev.emby.media/doc/restapi/index.html)

### 🛠️ 技术服务 (Technical Services)

#### EncodingInfoService
- **功能**: 编码信息服务
- **文档**: [EncodingInfoService](https://dev.emby.media/reference/RestAPI/EncodingInfoService.html)

#### CodecParameterService
- **功能**: 编解码器参数服务
- **文档**: [CodecParameterService](https://dev.emby.media/reference/RestAPI/CodecParameterService.html)

#### FfmpegOptionsService
- **功能**: FFmpeg 选项服务
- **文档**: [FfmpegOptionsService](https://dev.emby.media/reference/RestAPI/FfmpegOptionsService.html)

#### ToneMapOptionsService
- **功能**: 色调映射选项服务
- **文档**: [ToneMapOptionsService](https://dev.emby.media/reference/RestAPI/ToneMapOptionsService.html)

#### SubtitleOptionsService
- **功能**: 字幕选项服务
- **文档**: [SubtitleOptionsService](https://dev.emby.media/reference/RestAPI/SubtitleOptionsService.html)

### 🌐 网络和连接服务 (Network & Connectivity)

#### ConnectService
- **功能**: 连接服务
- **文档**: [ConnectService](https://dev.emby.media/reference/RestAPI/ConnectService.html)

#### DlnaService
- **功能**: DLNA 服务
- **文档**: [DlnaService](https://dev.emby.media/reference/RestAPI/DlnaService.html)

#### DlnaServerService
- **功能**: DLNA 服务器服务
- **文档**: [DlnaServerService](https://dev.emby.media/reference/RestAPI/DlnaServerService.html)

### 🔧 其他工具服务 (Utility Services)

#### LocalizationService
- **功能**: 本地化服务
- **文档**: [LocalizationService](https://dev.emby.media/reference/RestAPI/LocalizationService.html)

#### EnvironmentService
- **功能**: 环境服务
- **文档**: [EnvironmentService](https://dev.emby.media/reference/RestAPI/EnvironmentService.html)

#### FeatureService
- **功能**: 功能服务
- **文档**: [FeatureService](https://dev.emby.media/reference/RestAPI/FeatureService.html)

#### OfficialRatingService
- **功能**: 官方评级服务
- **文档**: [OfficialRatingService](https://dev.emby.media/reference/RestAPI/OfficialRatingService.html)

#### RemoteImageService
- **功能**: 远程图片服务
- **文档**: [RemoteImageService](https://dev.emby.media/reference/RestAPI/RemoteImageService.html)

#### BifService
- **功能**: BIF 服务 (缩略图)
- **文档**: [BifService](https://dev.emby.media/reference/RestAPI/BifService.html)

#### GenericUIApiService
- **功能**: 通用 UI API 服务
- **文档**: [GenericUIApiService](https://dev.emby.media/reference/RestAPI/GenericUIApiService.html)

#### OpenApiService
- **功能**: OpenAPI 服务
- **文档**: [OpenApiService](https://dev.emby.media/reference/RestAPI/OpenApiService.html)

---

## 🎯 实现优先级建议

### 🔥 高优先级 (MVP 必需)
标记为 ⭐ 的服务是实现基本功能的核心服务

### 📊 统计信息
- **总服务数**: 60+ 个
- **核心服务**: 15 个
- **可选服务**: 45+ 个

### 📚 相关资源
- **官方文档**: https://dev.emby.media/
- **API 浏览器**: http://swagger.emby.media/?staticview=true
- **SDK 下载**: https://dev.emby.media/download/index.html

---

*文档生成时间: 2025-01-27*
*基于 Emby Server 官方 API 文档整理*
