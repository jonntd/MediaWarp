# 运行阶段 - 使用alpine镜像（goreleaser预构建二进制文件）
FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

# 复制goreleaser构建的应用程序（rclone通过volume挂载）
COPY media-warp /media-warp

# 设置权限
RUN chmod +x /media-warp

# 环境变量
ENV GIN_MODE=release

# 数据卷（包含rclone二进制文件挂载点）
VOLUME ["/config", "/logs", "/custom", "/media", "/root/.config/rclone", "/usr/bin/rclone"]

# 暴露端口
EXPOSE 9096

# 启动命令
ENTRYPOINT ["/media-warp"]
