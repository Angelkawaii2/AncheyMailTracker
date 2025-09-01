########## base-build：构建基础（可缓存 go mod 依赖 + 构建依赖） ##########
FROM golang:1.25-bookworm AS base-build

# 构建期依赖（CGO & libheif等头文件）
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config cmake git \
    libwebp-dev libheif-dev libde265-dev libx265-dev \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN git clone --branch v1.20.2 --depth 1 https://github.com/strukturag/libheif.git \
 && cd libheif \
 && cmake -S . -B build -DCMAKE_BUILD_TYPE=Release \
 && cmake --build build -j"$(nproc)" \
 && cmake --install build && ldconfig

ENV CGO_ENABLED=1 \
    GO111MODULE=on
WORKDIR /app

# 仅复制依赖清单，保证 go mod download 这一层可复用缓存
COPY go.mod go.sum ./

# 使用 BuildKit 缓存 Go modules
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

########## dev：本地开发（热重载） ##########
FROM base-build AS dev

# 复制全量源码（仅 dev / builder 阶段）
COPY . .

# 开发工具（air）
RUN go install github.com/air-verse/air@latest

# 默认为热重载；生产用 builder/runtime
CMD ["air", "-c", ".air.toml"]

########## builder：用于生产编译 ##########
FROM base-build AS builder

# 复制全量源码（与 dev 分离，避免 dev 专用工具影响）
COPY . .

# 编译（使用 BuildKit 缓存 go-build 与 gomod）
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -v -o /out/app .

########## runtime：精简运行镜像 ##########
FROM debian:bookworm-slim AS runtime

# 仅安装运行期动态库与 CA
RUN apt-get update && apt-get install -y --no-install-recommends \
    libwebp7 libheif1 libde265-0 libx265-199 ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 拷贝编译产物
COPY --from=builder /out/app /app/app

# 拷贝模板与静态资源（若改用 go:embed，可删除这两行）
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/styles    /app/styles

ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/app/app"]
