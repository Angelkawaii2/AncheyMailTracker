# ---------- base：构建与开发环境 ----------
FROM golang:1.25-bookworm AS dev

# 在 dev/builder 阶段安装构建依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config cmake git \
     libwebp-dev \
     libheif-dev \
     libde265-dev libx265-dev \
    && rm -rf /var/lib/apt/lists/*

# 构建并安装 libheif 1.20.2（只需解码可不装 x265）
WORKDIR /tmp
RUN git clone --branch v1.20.2 --depth 1 https://github.com/strukturag/libheif.git \
 && cd libheif \
 && cmake -S . -B build -DCMAKE_BUILD_TYPE=Release \
 && cmake --build build -j"$(nproc)" \
 && cmake --install build \
 && ldconfig

ENV CGO_ENABLED=1
ENV GO111MODULE=on
WORKDIR /app

# 预拉模块缓存（加速构建）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 开发默认启用 air 热重载（下一步会放 .air.toml）
RUN go install github.com/air-verse/air@latest
CMD ["air", "-c", ".air.toml"]

# ---------- builder：用于生产构建 ----------
FROM dev AS builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -v -o /out/app .

# ---------- runtime：最小运行镜像（仅运行期依赖） ----------
FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    libwebp7 libheif1 libde265-0 libx265-199 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/app /app/app
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/app/app"]
