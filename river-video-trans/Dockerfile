FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o river-video-trans ./cmd/server

# BtbN static build includes NVENC + NVDEC (cuvid); Ubuntu's apt ffmpeg does not.
# Pinned to an immutable release rather than the drifting `latest`/`master-latest`:
# master builds silently bump the required NVENC SDK and can out-run the host
# NVIDIA driver (this build needs driver >= 610). To bump, pick a newer tag+asset
# from https://github.com/BtbN/FFmpeg-Builds/releases and re-verify NVENC on the host.
FROM alpine:3.21 AS ffmpeg-downloader
ARG FFMPEG_RELEASE=autobuild-2026-08-01-13-21
ARG FFMPEG_BUILD=ffmpeg-N-125881-g946272b79a-linux64-gpl
RUN apk add --no-cache curl xz
RUN curl -fsSL -o /tmp/ffmpeg.tar.xz \
    https://github.com/BtbN/FFmpeg-Builds/releases/download/${FFMPEG_RELEASE}/${FFMPEG_BUILD}.tar.xz && \
    tar -xJf /tmp/ffmpeg.tar.xz --strip-components=2 -C /usr/local/bin \
        ${FFMPEG_BUILD}/bin/ffmpeg \
        ${FFMPEG_BUILD}/bin/ffprobe && \
    rm /tmp/ffmpeg.tar.xz

# nvidia/cuda runtime supplies libnvcuvid.so (NVDEC) and libnvidia-encode.so (NVENC)
# via nvidia-container-toolkit at runtime.
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*
COPY --from=ffmpeg-downloader /usr/local/bin/ffmpeg  /usr/local/bin/ffmpeg
COPY --from=ffmpeg-downloader /usr/local/bin/ffprobe /usr/local/bin/ffprobe
WORKDIR /app
COPY --from=builder /app/river-video-trans .
CMD ["./river-video-trans"]
