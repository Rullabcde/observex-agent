#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-latest}"
AGENT_DIR="/opt/uptimeid"
BINARY="${AGENT_DIR}/uptimeid-agent"
SERVICE_USER="${SERVICE_USER:-root}"

# --- Functions ---
info()  { printf "\033[1;34m[INFO]\033[0m %s\n" "$*"; }
ok()    { printf "\033[1;32m[ OK ]\033[0m %s\n" "$*"; }
err()   { printf "\033[1;31m[ERR ]\033[0m %s\n" "$*"; exit 1; }

detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) err "Unsupported architecture: $arch" ;;
    esac
}

detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      err "Unsupported OS: $(uname -s)" ;;
    esac
}

# --- Main ---
OS="$(detect_os)"
ARCH="$(detect_arch)"
BINARY_NAME="uptimeid-agent-${OS}-${ARCH}"

info "Detected: ${OS} / ${ARCH}"
info "Installing UptimeID Agent to ${AGENT_DIR}"

# Create agent directory
mkdir -p "${AGENT_DIR}"

# Download binary
if [ "${VERSION}" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/uptime-id/agent/releases/latest/download/${BINARY_NAME}"
else
    DOWNLOAD_URL="https://github.com/uptime-id/agent/releases/download/${VERSION}/${BINARY_NAME}"
fi

info "Downloading from ${DOWNLOAD_URL}"
if command -v curl &>/dev/null; then
    curl -fsSL -o "${BINARY}" "${DOWNLOAD_URL}"
elif command -v wget &>/dev/null; then
    wget -qO "${BINARY}" "${DOWNLOAD_URL}"
else
    err "Neither curl nor wget found"
fi

# --- CRITICAL: Set execute permission ---
chmod 755 "${BINARY}"
ok "Set execute permission on ${BINARY}"

# Verify binary
if ! file "${BINARY}" | grep -qi "executable"; then
    err "Downloaded file is not an executable — corrupt download?"
fi
info "Binary: $(file "${BINARY}")"

# Copy .env.example if no .env exists
if [ -f ".env.example" ]; then
    if [ ! -f "${AGENT_DIR}/.env" ]; then
        cp ".env.example" "${AGENT_DIR}/.env"
        info "Created ${AGENT_DIR}/.env — edit it with your API_KEY and API_URL"
    else
        info "${AGENT_DIR}/.env already exists, skipping"
    fi
fi

# --- Platform-specific setup ---
case "${OS}" in
    linux)
        # Install systemd service
        if command -v systemctl &>/dev/null; then
            SERVICE_SRC="platforms/linux/uptimeid-agent.service"
            SERVICE_DST="/etc/systemd/system/uptimeid-agent.service"

            if [ -f "${SERVICE_SRC}" ]; then
                cp "${SERVICE_SRC}" "${SERVICE_DST}"
                chmod 644 "${SERVICE_DST}"
                systemctl daemon-reload
                systemctl enable uptimeid-agent
                systemctl restart uptimeid-agent
                ok "Installed and started systemd service"
            else
                info "Service file not found at ${SERVICE_SRC}, skipping"
            fi
        else
            info "systemctl not found, skipping service installation"
        fi
        ;;

    darwin)
        # Install launchd plist
        PLIST_SRC="platforms/darwin/com.uptimeid.agent.plist"
        PLIST_DST="/Library/LaunchDaemons/com.uptimeid.agent.plist"

        if [ -f "${PLIST_SRC}" ]; then
            cp "${PLIST_SRC}" "${PLIST_DST}"
            chmod 644 "${PLIST_DST}"
            launchctl load "${PLIST_DST}"
            ok "Installed and loaded launchd daemon"
        else
            info "Plist file not found at ${PLIST_SRC}, skipping"
        fi
        ;;
esac

ok "UptimeID Agent ${VERSION} installed successfully!"
info "Edit ${AGENT_DIR}/.env with your API_KEY and API_URL, then restart the service."
