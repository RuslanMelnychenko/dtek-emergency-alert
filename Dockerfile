# Stage 1: Modules caching
FROM golang:1.25 as modules
COPY go.mod go.sum /modules/
WORKDIR /modules
RUN go mod download

# Stage 2: Build
FROM --platform=$BUILDPLATFORM golang:1.25 as builder
ARG TARGETOS
ARG TARGETARCH
COPY --from=modules /go/pkg /go/pkg
COPY . /workdir
WORKDIR /workdir
# Install playwright cli with right version for later use
RUN PWGO_VER=$(grep -oE "playwright-go v\S+" /workdir/go.mod | sed 's/playwright-go //g') \
    && GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /bin/playwright github.com/playwright-community/playwright-go/cmd/playwright
# Build your app
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /bin/myapp ./cmd/bot/main.go

# Stage 3: Final
FROM ubuntu:noble
COPY --from=builder /bin/playwright /bin/myapp /
RUN apt-get update && apt-get install -y ca-certificates tzdata \
    && /playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*
ENTRYPOINT ["./myapp"]
CMD ["bot-checking"]
