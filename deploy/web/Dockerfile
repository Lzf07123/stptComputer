# ---- 前端构建 ----
FROM node:20-alpine AS fe
WORKDIR /fe
COPY frontend/package.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- nginx 托管静态资源 + 反代 ----
FROM nginx:1.27-alpine
COPY deploy/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=fe /fe/dist /usr/share/nginx/html
EXPOSE 8080
