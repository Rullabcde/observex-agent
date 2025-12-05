# FROM golang:1.22-alpine AS builder
# WORKDIR /app
# RUN apk add --no-cache git ca-certificates tzdata
# COPY go.mod go.sum ./
# RUN go mod download
# COPY . .
# ARG VERSION=dev
# ARG COMMIT=none
# ARG DATE=unknown

# RUN CGO_ENABLED=0 GOOS=linux go build \
#     -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
#     -o /agent main.go

# FROM alpine:3.19
# WORKDIR /app
# RUN apk add --no-cache ca-certificates tzdata
# COPY --from=builder /agent .
# RUN adduser -D -u 1001 agent
# USER agent
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD pgrep agent || exit 1
# ENTRYPOINT ["/app/agent"]

# Build Debian 11 (GLIBC 2.31)
FROM golang:1.22-bullseye AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -s -w -extldflags '-static'" \
    -tags netgo,osusergo \
    -installsuffix netgo \
    -o observex-agent \
    main.go

RUN file observex-agent && \
    ldd observex-agent 2>&1 | grep -q "not a dynamic executable" || (echo "Warning: Not a static binary" && exit 0)

# Runtime Debian 11 (GLIBC 2.31)
FROM debian:11-slim AS runtime-debian
WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*


COPY --from=builder /build/observex-agent .

RUN chmod +x observex-agent
ENTRYPOINT ["./observex-agent"]

# Alpine (musl libc, tidak tergantung GLIBC)
FROM alpine:3.18 AS runtime-alpine
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/observex-agent .
RUN chmod +x observex-agent
ENTRYPOINT ["./observex-agent"]

# Scratch
FROM scratch AS runtime-scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/observex-agent /observex-agent
ENTRYPOINT ["/observex-agent"]
FROM runtime-alpine AS final