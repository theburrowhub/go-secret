#!/usr/bin/env bash
# go-secrets installer
# Usage:
#   Remote: curl -sSL https://raw.githubusercontent.com/theburrowhub/go-secret/main/install.sh | bash
#   Local:  ./install.sh

set -euo pipefail

REPO="theburrowhub/go-secret"
BINARY_NAME="go-secrets"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info() { echo -e "${CYAN}▶${RESET} $1"; }
success() { echo -e "${GREEN}✓${RESET} $1"; }
warn() { echo -e "${YELLOW}⚠${RESET} $1"; }
error() { echo -e "${RED}✗${RESET} $1" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "darwin" ;;
        Linux*)  echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

get_install_dir() {
    if [[ -w "/usr/local/bin" ]]; then
        echo "/usr/local/bin"
    elif [[ -d "$HOME/.local/bin" ]] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
        echo "$HOME/.local/bin"
    else
        error "Cannot find writable install directory"
    fi
}

is_local_repo() {
    [[ -f "go.mod" ]] && grep -q "github.com/theburrowhub/go-secret" go.mod 2>/dev/null
}

install_from_source() {
    info "Detected local repository, building from source..."
    
    if ! command -v go &>/dev/null; then
        error "Go is not installed. Please install Go 1.21+ first."
    fi
    
    local install_dir
    install_dir=$(get_install_dir)
    
    info "Building ${BINARY_NAME}..."
    go build -trimpath -ldflags "-s -w" -o "${BINARY_NAME}" .
    
    info "Installing to ${install_dir}..."
    mv "${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
    chmod +x "${install_dir}/${BINARY_NAME}"
    
    success "Installed ${BINARY_NAME} to ${install_dir}/${BINARY_NAME}"
    check_path "${install_dir}"
}

install_from_release() {
    info "Installing ${BINARY_NAME} from GitHub releases..."
    
    local os arch install_dir version download_url tmp_dir
    os=$(detect_os)
    arch=$(detect_arch)
    install_dir=$(get_install_dir)
    
    info "Detecting latest version..."
    version=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [[ -z "$version" ]]; then
        error "Could not detect latest version. Check https://github.com/${REPO}/releases"
    fi
    
    info "Latest version: ${version}"
    
    local archive_name="${BINARY_NAME}_${version#v}_${os}_${arch}.tar.gz"
    if [[ "$os" == "windows" ]]; then
        archive_name="${BINARY_NAME}_${version#v}_${os}_${arch}.zip"
    fi
    
    download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    
    info "Downloading ${archive_name}..."
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT
    
    if ! curl -sSL -o "${tmp_dir}/${archive_name}" "$download_url"; then
        error "Failed to download from ${download_url}"
    fi
    
    info "Extracting..."
    if [[ "$os" == "windows" ]]; then
        unzip -q "${tmp_dir}/${archive_name}" -d "${tmp_dir}"
    else
        tar -xzf "${tmp_dir}/${archive_name}" -C "${tmp_dir}"
    fi
    
    info "Installing to ${install_dir}..."
    mv "${tmp_dir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
    chmod +x "${install_dir}/${BINARY_NAME}"
    
    success "Installed ${BINARY_NAME} ${version} to ${install_dir}/${BINARY_NAME}"
    check_path "${install_dir}"
}

check_path() {
    local install_dir="$1"
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        warn "${install_dir} is not in your PATH"
        echo ""
        echo -e "${BOLD}Add to your shell config:${RESET}"
        echo -e "  ${CYAN}export PATH=\"\$PATH:${install_dir}\"${RESET}"
        echo ""
    fi
}

main() {
    echo ""
    echo -e "${BOLD}${CYAN}🔐 go-secrets installer${RESET}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo ""
    
    if is_local_repo; then
        install_from_source
    else
        install_from_release
    fi
    
    echo ""
    echo -e "${GREEN}${BOLD}Installation complete!${RESET}"
    echo -e "Run ${CYAN}${BINARY_NAME}${RESET} to start."
    echo ""
}

main "$@"

