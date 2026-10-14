# 基础镜像与软件源均可由构建参数/环境变量控制
ARG NODE_IMAGE=node:20-alpine
ARG NGINX_IMAGE=nginx:1.27-alpine
ARG NPM_REGISTRY=https://registry.npmjs.org/

# ---- 前端构建 ----
FROM ${NODE_IMAGE} AS fe
ARG NPM_REGISTRY
WORKDIR /fe
RUN npm config set registry ${NPM_REGISTRY}
COPY frontend/package.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- nginx 托管静态资源 + 反代 ----
FROM ${NGINX_IMAGE}
COPY deploy/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=fe /fe/dist /usr/share/nginx/html
EXPOSE 8080
