# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG SERVICE
RUN test -n "$SERVICE"
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/${SERVICE} ./cmd/${SERVICE}

FROM alpine:3.22 AS backend
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ARG SERVICE
COPY --from=backend-builder /out/${SERVICE} /app/service
COPY etc /app/etc
EXPOSE 8080 8081 8082 8083 8084 8085 8086 9090 9091 9092 9093 9094 9095
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
