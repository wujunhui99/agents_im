# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG SERVICE
RUN test -n "$SERVICE"
# Map deployment name -> go-zero main package path. Entrypoints live in their
# service directories (cmd/ was removed); deploy still passes SERVICE=<name>.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    case "$SERVICE" in \
      agent-api)        pkg=./service/agent/api ;; \
      auth-api)         pkg=./service/auth/api ;; \
      auth-rpc)         pkg=./service/auth/rpc ;; \
      friends-api)      pkg=./service/friends/api ;; \
      friends-rpc)      pkg=./service/friends/rpc ;; \
      groups-api)       pkg=./service/groups/api ;; \
      groups-rpc)       pkg=./service/groups/rpc ;; \
      third-rpc)         pkg=./service/third/rpc ;; \
      user-api)         pkg=./service/user/api ;; \
      user-rpc)         pkg=./service/user/rpc ;; \
      media-api)        pkg=./service/media/api ;; \
      media-rpc)        pkg=./service/media/rpc ;; \
      message-rpc)      pkg=./internal/rpcgen/message ;; \
      gateway-ws)       pkg=./service/gateway-ws ;; \
      admin-api)        pkg=./service/admin/api ;; \
      admin-rpc)        pkg=./service/admin/rpc ;; \
      message-api)      pkg=./service/message-api ;; \
      message-transfer) pkg=./service/message-transfer ;; \
      *) echo "unknown SERVICE: $SERVICE" >&2; exit 1 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/service "$pkg"

FROM alpine:3.22 AS backend
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ARG SERVICE
RUN test -n "$SERVICE"
COPY --from=backend-builder /out/service /app/service
COPY etc /app/etc
EXPOSE 8080 8081 8082 8083 8084 8085 8086 8088 9090 9091 9092 9093 9094 9095 9097
ENTRYPOINT ["/app/service"]

FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web ./
ARG VITE_API_BASE_URL=/
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

FROM nginx:1.29-alpine AS web
COPY web/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=web-builder /src/web/dist /usr/share/nginx/html
EXPOSE 80
