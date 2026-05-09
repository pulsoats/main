# syntax=docker/dockerfile:1.7

########################
# Builder
########################
FROM golang:1.25 AS builder

WORKDIR /workspace/main

ENV GOPRIVATE=github.com/pulsoats/* \
    GONOSUMDB=github.com/pulsoats/* \
    GONOPROXY=github.com/pulsoats/*

RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Модули (сохраняем отдельным слоем для кэша)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=github_token \
    set -e; \
    if [ -f /run/secrets/github_token ]; then \
      printf "machine github.com\nlogin %s\npassword x-oauth-basic\n" \
        "$(cat /run/secrets/github_token)" > /root/.netrc; \
      chmod 600 /root/.netrc; \
    fi; \
    go mod download; \
    rm -f /root/.netrc

# Код
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build -o /workspace/bin/pulsoats-main ./cmd

########################
# Runtime
########################
FROM gcr.io/distroless/base-debian12:nonroot AS runner

WORKDIR /app

COPY --from=builder /workspace/bin/pulsoats-main /usr/local/bin/pulsoats-main

ENTRYPOINT ["/usr/local/bin/pulsoats-main"]
