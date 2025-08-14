package constants

type MediaServerType string // 媒体服务器类型

const (
	EMBY MediaServerType = "Emby" // 媒体服务器类型：EmbyServer
	PLEX MediaServerType = "Plex" // 媒体服务器类型：Plex
)
