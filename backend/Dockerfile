# 基础镜像与软件源可变；软件源统一由 SOURCE_MIRROR 控制（空=官方；cn=国内镜像组合）
ARG GOLANG_IMAGE=golang:1.22-alpine
ARG ALPINE_IMAGE=alpine:3.20
ARG SOURCE_MIRROR=

# ---- build ----
FROM ${GOLANG_IMAGE} AS build
ARG SOURCE_MIRROR
WORKDIR /src
COPY go.mod go.sum* ./
RUN if [ "$SOURCE_MIRROR" = "cn" ]; then \
        go env -w GOPROXY=https://goproxy.cn,direct; \
    fi
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime ----
FROM ${ALPINE_IMAGE}
ARG SOURCE_MIRROR
RUN if [ "$SOURCE_MIRROR" = "cn" ]; then \
        sed -i "s@dl-cdn.alpinelinux.org@mirrors.aliyun.com@g" /etc/apk/repositories; \
    fi \
    && apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/server .
EXPOSE 8080
ENTRYPOINT ["./server"]
