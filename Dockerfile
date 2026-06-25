# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src
RUN apk add --no-cache git python3
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# Build all backend entrypoints once. Deployment manifests select the concrete
# binary with container command, so the CI path pushes one backend image instead
# of one image per backend service.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build <<'EOF'
set -eu
mkdir -p /out/bin
python3 - <<'PY' | while read -r name pkg; do
import json
from pathlib import Path

registry = json.loads(Path("scripts/services.json").read_text())
for service in registry["backend"]:
    print(service["name"], service["package"])
PY
  echo "building ${name} from ${pkg}"
  CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o "/out/bin/${name}" "${pkg}"
done
EOF

FROM alpine:3.22 AS backend
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /out/bin /app/bin
COPY etc /app/etc
EXPOSE 8080 8081 8082 8083 8084 8085 8086 8088 9090 9091 9092 9093 9094 9095 9097 9098 9099 9100
ENTRYPOINT ["/app/bin/user-api"]

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
