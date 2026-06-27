# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src
RUN apk add --no-cache git python3
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG SERVICE
RUN test -n "$SERVICE"
# Build only the detector-selected service for this image. The package mapping
# stays in scripts/services.json so Docker, local tooling, and deploy selection
# share one service registry.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build <<'EOF'
set -eu
pkg="$(python3 - "$SERVICE" <<'PY'
import json
import sys
from pathlib import Path

registry = json.loads(Path("scripts/services.json").read_text())
for service in registry["backend"]:
    if service["name"] == sys.argv[1]:
        print(service["package"])
        break
else:
    raise SystemExit(f"unknown backend service: {sys.argv[1]}")
PY
)"
echo "building ${SERVICE} from ${pkg}"
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/service "${pkg}"
EOF

FROM alpine:3.22 AS backend
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /out/service /app/service
COPY etc /app/etc
EXPOSE 8080 8081 8082 8083 8084 8085 8086 8088 9090 9091 9092 9093 9094 9095 9097 9098 9099 9100
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
