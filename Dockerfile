FROM golang:1.24 AS builder

WORKDIR /app

# Download go modules
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on go mod download

COPY . .

RUN CGO_ENABLED=0 go build -a --trimpath --installsuffix cgo --ldflags="-s -w" -o media-warp

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
COPY --from=builder /app/media-warp /media-warp 
ENV GIN_MODE=release
RUN chmod +x /media-warp
VOLUME ["/config", "/logs", "/custom", "/media", "/root/.config/rclone"]
ENTRYPOINT ["/media-warp"]
EXPOSE 9096
