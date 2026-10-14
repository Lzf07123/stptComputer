# 基础镜像统一由 IMAGE_REGISTRY 控制 registry 前缀（空=官方 Docker Hub；可指向内网/镜像仓库，需以 / 结尾）
# 软件源统一由 SOURCE_MIRROR 控制（空=官方；cn=国内镜像组合）
ARG IMAGE_REGISTRY=
ARG SOURCE_MIRROR=

# ---- 前端构建 ----
FROM ${IMAGE_REGISTRY}node:20-alpine AS fe
ARG SOURCE_MIRROR
WORKDIR /fe
RUN if [ "$SOURCE_MIRROR" = "cn" ]; then \
        npm config set registry https://registry.npmmirror.com/; \
    fi
COPY frontend/package.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- nginx 托管静态资源 + 反代 ----
FROM ${IMAGE_REGISTRY}nginx:1.27-alpine
COPY deploy/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=fe /fe/dist /usr/share/nginx/html
EXPOSE 8080
