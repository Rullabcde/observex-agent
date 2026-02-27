FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories && \
    apk add --no-cache git ca-certificates tzdata upx
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /agent main.go
RUN upx --best --lzma /agent

RUN echo "root:x:0:0:root:/root:/bin/sh" > /etc/passwd.scratch && \
    echo "agent:x:1001:1001:UptimeID Agent:/nonexistent:/sbin/nologin" >> /etc/passwd.scratch && \
    echo "root:x:0:" > /etc/group.scratch && \
    echo "agent:x:1001:" >> /etc/group.scratch

FROM scratch
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd.scratch /etc/passwd
COPY --from=builder /etc/group.scratch /etc/group
COPY --from=builder /agent /agent

USER 1001
ENTRYPOINT ["/agent"]
