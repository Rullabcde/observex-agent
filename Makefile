VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BINARY  := uptimeid-agent

.PHONY: all build clean install install-linux install-darwin test

all: build

# --- Build ---
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) main.go

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-amd64 main.go

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-arm64 main.go

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-darwin-amd64 main.go

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-darwin-arm64 main.go

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe main.go

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows

# --- Install ---
install: build
	@echo "==> Installing to /opt/uptimeid/..."
	mkdir -p /opt/uptimeid
	cp $(BINARY) /opt/uptimeid/$(BINARY)
	chmod 755 /opt/uptimeid/$(BINARY)        # <-- set execute permission
	[ ! -f /opt/uptimeid/.env ] && cp -n .env.example /opt/uptimeid/.env 2>/dev/null || true
	@echo "==> Done. Edit /opt/uptimeid/.env and run: sudo systemctl start uptimeid-agent"

install-linux: install
	@echo "==> Installing systemd service..."
	cp platforms/linux/uptimeid-agent.service /etc/systemd/system/
	chmod 644 /etc/systemd/system/uptimeid-agent.service
	systemctl daemon-reload
	systemctl enable uptimeid-agent
	systemctl restart uptimeid-agent
	@echo "==> Service installed and started."

install-darwin: build
	@echo "==> Installing to /opt/uptimeid/..."
	mkdir -p /opt/uptimeid
	cp $(BINARY) /opt/uptimeid/$(BINARY)
	chmod 755 /opt/uptimeid/$(BINARY)
	[ ! -f /opt/uptimeid/.env ] && cp -n .env.example /opt/uptimeid/.env 2>/dev/null || true
	@echo "==> Installing launchd plist..."
	cp platforms/darwin/com.uptimeid.agent.plist /Library/LaunchDaemons/
	chmod 644 /Library/LaunchDaemons/com.uptimeid.agent.plist
	launchctl load /Library/LaunchDaemons/com.uptimeid.agent.plist
	@echo "==> Service installed and loaded."

# --- Clean ---
clean:
	rm -f $(BINARY) $(BINARY)-*

test:
	go test ./...
