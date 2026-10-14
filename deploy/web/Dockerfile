# 基础镜像与软件源可变；软件源统一由 SOURCE_MIRROR 控制（空=官方；cn=国内镜像组合）
ARG NODE_IMAGE=node:20-alpine
ARG NGINX_IMAGE=nginx:1.27-alpine
ARG SOURCE_MIRROR=

# ---- 前端构建 ----
FROM ${NODE_IMAGE} AS fe
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
FROM ${NGINX_IMAGE}
COPY deploy/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=fe /fe/dist /usr/share/nginx/html
EXPOSE 8080
