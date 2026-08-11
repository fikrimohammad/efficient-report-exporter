# syntax=docker/dockerfile:1

# ── Build stage ─────────────────────────────────────────────────────────────
# Static, stripped builds of both service binaries. CGO is disabled so the
# output is a self-contained binary that runs on any base image.
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module resolution: the repo vendors its dependencies, so layer the
# manifests and vendor tree first; only source changes invalidate the build.
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=vendor -trimpath \
      -ldflags "-s -w -X github.com/fikrimohammad/efficient-report-exporter/common/appinfo.version=${VERSION}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=vendor -trimpath \
      -ldflags "-s -w -X github.com/fikrimohammad/efficient-report-exporter/common/appinfo.version=${VERSION}" \
      -o /out/mq ./cmd/mq

# ── API image ───────────────────────────────────────────────────────────────
# distroless: ~2MB OS, no shell, runs as non-root. The API listens on 18081
# (config/api_server.addr) and reads config/config.<APP_ENV>.yaml from /app.
# Mount the real config over /app/config at runtime.
FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /src/config /app/config
EXPOSE 18081
ENTRYPOINT ["/app/api"]

# ── MQ consumer image ───────────────────────────────────────────────────────
# Same runtime base; the consumer worker has no exposed ports.
FROM gcr.io/distroless/static-debian12:nonroot AS mq
WORKDIR /app
COPY --from=builder /out/mq /app/mq
COPY --from=builder /src/config /app/config
ENTRYPOINT ["/app/mq"]
