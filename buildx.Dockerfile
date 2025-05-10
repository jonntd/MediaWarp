FROM golang:1.24 AS builder

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

FROM ghcr.io/by275/base:ubuntu24.04 AS base
FROM base AS rclone
ARG APT_MIRROR="archive.ubuntu.com"

RUN echo "**** apt source change for local build ****" && \
    sed -i "s/archive.ubuntu.com/archive.ubuntu.com/g" /etc/apt/sources.list && \
    echo "**** add rclone ****" && \
    apt-get update -qq && \
    apt-get install -yq --no-install-recommends unzip && \
    rclone_install_script_url="https://raw.githubusercontent.com/jonntd/rclone/master-115/install.sh" && \
    curl -fsSL $rclone_install_script_url | bash

# 使用alpine作为最小基础镜像而不是scratch
FROM alpine:latest
# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

COPY --from=rclone /usr/bin/rclone /usr/bin/
# COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY media-warp /
RUN chmod +x /media-warp
ENV GIN_MODE=release
VOLUME ["/config", "/logs", "/custom", "/media", "/root/.config/rclone"]

ENTRYPOINT ["/media-warp"]
EXPOSE 9096
