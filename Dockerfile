FROM golang:1.24 AS builder

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# Download go modules
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on go mod download

COPY . .

RUN CGO_ENABLED=0 go build -a --trimpath --installsuffix cgo --ldflags="-s" -o media-warp

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

# Create a minimal container to run a Golang static binary
FROM scratch
COPY --from=rclone /usr/bin/rclone /usr/bin/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/media-warp .
COPY --from=builder /app/static /static
ENV GIN_MODE=release
ENTRYPOINT ["/media-warp"]
EXPOSE 9096
