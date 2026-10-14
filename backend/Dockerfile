# 基础镜像与软件源均可由构建参数/环境变量控制（默认官方源，国内可切镜像）
ARG GOLANG_IMAGE=golang:1.22-alpine
ARG ALPINE_IMAGE=alpine:3.20
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ARG APK_MIRROR=

# ---- build ----
FROM ${GOLANG_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum* ./
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime ----
FROM ${ALPINE_IMAGE}
ARG APK_MIRROR
RUN if [ -n "$APK_MIRROR" ]; then \
        sed -i "s@dl-cdn.alpinelinux.org@${APK_MIRROR}@g" /etc/apk/repositories; \
    fi \
    && apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/server .
EXPOSE 8080
ENTRYPOINT ["./server"]
