#!/usr/bin/env bash
set -euo pipefail

REPO="${DEVSCOPE_REPO:-PirataZang/devscope}"
VERSION="${DEVSCOPE_VERSION:-latest}"
INSTALL_DIR="${DEVSCOPE_INSTALL_DIR:-}"

err() {
	echo "error: $*" >&2
	exit 1
}

info() {
	echo "==> $*"
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || err "comando obrigatório não encontrado: $1"
}

need_cmd curl
need_cmd tar

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
linux) OS=linux ;;
darwin) OS=darwin ;;
*) err "sistema operacional não suportado: $OS (use: go install github.com/${REPO}/cmd/devscope@latest)" ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) err "arquitetura não suportada: $ARCH" ;;
esac

if [[ -z "$INSTALL_DIR" ]]; then
	if [[ -w /usr/local/bin ]] 2>/dev/null; then
		INSTALL_DIR=/usr/local/bin
	elif mkdir -p "$HOME/.local/bin" 2>/dev/null; then
		INSTALL_DIR="$HOME/.local/bin"
	else
		INSTALL_DIR=/usr/local/bin
	fi
fi

mkdir -p "$INSTALL_DIR"

if [[ "$VERSION" == "latest" ]]; then
	VERSION=$(
		curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
			sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' |
			head -1
	)
	[[ -n "$VERSION" ]] || err "não foi possível obter a versão mais recente (release publicada?)"
fi

TAG="$VERSION"
[[ "$TAG" == v* ]] || TAG="v${TAG}"
VER="${TAG#v}"

ASSET="devscope_${VER}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

info "baixando ${ASSET}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP}/${ASSET}"

if curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP}/checksums.txt" 2>/dev/null; then
	EXPECTED=$(grep " ${ASSET}$" "${TMP}/checksums.txt" | awk '{print $1}')
	if [[ -n "$EXPECTED" ]]; then
		need_cmd sha256sum
		ACTUAL=$(sha256sum "${TMP}/${ASSET}" | awk '{print $1}')
		[[ "$ACTUAL" == "$EXPECTED" ]] || err "checksum inválido"
		info "checksum ok"
	fi
fi

tar -xzf "${TMP}/${ASSET}" -C "$TMP"
BINARY="${TMP}/devscope"
[[ -x "$BINARY" ]] || err "binário não encontrado no arquivo"

if [[ -w "$INSTALL_DIR" ]]; then
	install -m 755 "$BINARY" "${INSTALL_DIR}/devscope"
else
	need_cmd sudo
	sudo install -m 755 "$BINARY" "${INSTALL_DIR}/devscope"
fi

info "instalado em ${INSTALL_DIR}/devscope"

if ! command -v devscope >/dev/null 2>&1; then
	echo "adicione ao PATH: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

if command -v devscope >/dev/null 2>&1; then
	devscope version
fi
