# syntax=docker/dockerfile:1.7

########################
# Builder
########################
FROM golang:1.25 AS builder

WORKDIR /workspace/main

ARG GITHUB_TOKEN
ENV GOPRIVATE=github.com/pulsoats/*

RUN if [ -n "$GITHUB_TOKEN" ]; then \
      git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"; \
    fi

# Модули (сохраняем отдельным слоем для кэша)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Код
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build -o /workspace/bin/pulsoats-main ./cmd

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1

########################
# Runtime
########################
FROM gcr.io/distroless/base-debian12:nonroot AS runner

WORKDIR /app

COPY --from=builder /workspace/bin/pulsoats-main /usr/local/bin/pulsoats-main
COPY --from=builder /workspace/main/migrations ./migrations
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate

ENTRYPOINT ["/usr/local/bin/pulsoats-main"]
