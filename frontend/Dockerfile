# ---- Build stage ----
FROM node:20-alpine AS builder
WORKDIR /app

# Copy package files first for cache-efficient layers
COPY package.json package-lock.json ./
RUN npm ci

# Copy sources and build
COPY tsconfig*.json vite.config.ts index.html ./
COPY public ./public
COPY src ./src
RUN npm run build

# ---- Runtime stage ----
FROM nginx:alpine AS runtime

# wget for healthcheck compatibility with rest of stack
RUN apk add --no-cache wget

# Override default nginx config with our SPA + proxy config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Built artifacts
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
# nginx:alpine default CMD: ["nginx", "-g", "daemon off;"] — inherit
