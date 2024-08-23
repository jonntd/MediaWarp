# syntax=docker/dockerfile:1.2
FROM golang:1-alpine as builder

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
    rclone_install_script_url="https://raw.githubusercontent.com/wiserain/rclone/mod/install.sh" && \
    curl -fsSL $rclone_install_script_url | bash


# Create a minimal container to run a Golang static binary
FROM scratch
COPY --from=rclone /usr/bin/rclone /usr/bin/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY MediaWarp /
ENV GIN_MODE=release
ENTRYPOINT ["/MediaWarp"]
EXPOSE 9096
