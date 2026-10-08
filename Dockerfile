# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/playwright github.com/mxschmitt/playwright-go/cmd/playwright \
 && go build -trimpath -ldflags="-s -w" -o /out/dtek-bot ./cmd/bot

FROM ubuntu:noble
ARG DEBIAN_FRONTEND=noninteractive
# Kept above the bot binary so code changes don't invalidate this layer; --only-shell is enough as the bot runs headless
RUN --mount=type=bind,from=builder,source=/out/playwright,target=/tmp/playwright \
    apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata tini \
 && /tmp/playwright install --with-deps --only-shell chromium \
 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/dtek-bot /usr/local/bin/dtek-bot
# The app writes to the relative data/ dir, which resolves to the /data volume
WORKDIR /
ENTRYPOINT ["tini", "--", "dtek-bot"]
CMD ["bot-checking"]
