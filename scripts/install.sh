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

# Detecta o comando correto de sha256 (Linux vs macOS)
sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		echo ""
	fi
}

need_cmd curl

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
linux)
	OS=linux
	EXT="tar.gz"
	BIN_NAME="devscope"
	;;
darwin)
	OS=darwin
	EXT="tar.gz"
	BIN_NAME="devscope"
	;;
mingw* | msys* | cygwin*)
	OS=windows
	EXT="zip"
	BIN_NAME="devscope.exe"
	;;
*)
	err "sistema operacional não suportado: $OS"
	;;
esac

ARCH=$(uname -m)
case "$ARCH" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) err "arquitetura não suportada: $ARCH" ;;
esac

if [[ "$EXT" == "tar.gz" ]]; then
	need_cmd tar
fi

if [[ -z "$INSTALL_DIR" ]]; then
	if [[ "$OS" == "windows" ]]; then
		INSTALL_DIR="$HOME/.local/bin"
	else
		if [[ -w /usr/local/bin ]] 2>/dev/null; then
			INSTALL_DIR=/usr/local/bin
		elif mkdir -p "$HOME/.local/bin" 2>/dev/null; then
			INSTALL_DIR="$HOME/.local/bin"
		else
			INSTALL_DIR=/usr/local/bin
		fi
	fi
fi

mkdir -p "$INSTALL_DIR"

if [[ "$VERSION" == "latest" ]]; then
	info "obtendo a versão mais recente..."
	VERSION=$(
		curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
			sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' |
			head -1
	)
	[[ -n "$VERSION" ]] || err "não foi possível obter a versão mais recente. Existe uma release publicada em https://github.com/${REPO}/releases?"
fi

TAG="$VERSION"
[[ "$TAG" == v* ]] || TAG="v${TAG}"
VER="${TAG#v}"

ASSET="devscope_${VER}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

info "baixando DevScope ${TAG} para ${OS}/${ARCH}..."
if ! curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP}/${ASSET}"; then
	err "falha ao baixar '${ASSET}'. Verifique se a release '${TAG}' existe em https://github.com/${REPO}/releases"
fi

# Verificação de checksum (opcional — continua se checksums.txt não existir)
if curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP}/checksums.txt" 2>/dev/null; then
	EXPECTED=$(grep " ${ASSET}$" "${TMP}/checksums.txt" | awk '{print $1}')
	if [[ -n "$EXPECTED" ]]; then
		ACTUAL=$(sha256 "${TMP}/${ASSET}")
		if [[ -n "$ACTUAL" ]]; then
			[[ "$ACTUAL" == "$EXPECTED" ]] || err "checksum inválido — o download pode estar corrompido"
			info "checksum verificado ✓"
		else
			info "aviso: nenhuma ferramenta de sha256 encontrada, pulando verificação de checksum"
		fi
	fi
fi

if [[ "$EXT" == "zip" ]]; then
	if command -v unzip >/dev/null 2>&1; then
		unzip -q "${TMP}/${ASSET}" -d "$TMP"
	else
		WIN_ZIP=$(cygpath -w "${TMP}/${ASSET}" 2>/dev/null || echo "${TMP}/${ASSET}")
		WIN_TMP=$(cygpath -w "${TMP}" 2>/dev/null || echo "${TMP}")
		powershell.exe -NoProfile -Command "Expand-Archive -Path '${WIN_ZIP}' -DestinationPath '${WIN_TMP}' -Force"
	fi
else
	tar -xzf "${TMP}/${ASSET}" -C "$TMP"
fi

# Busca o binário em qualquer subdiretório extraído
BINARY=$(find "$TMP" -name "$BIN_NAME" -type f | head -1)
[[ -n "$BINARY" && -f "$BINARY" ]] || err "binário '${BIN_NAME}' não encontrado no arquivo extraído"

if [[ "$OS" == "windows" ]]; then
	cp "$BINARY" "${INSTALL_DIR}/${BIN_NAME}"
else
	if [[ -w "$INSTALL_DIR" ]]; then
		install -m 755 "$BINARY" "${INSTALL_DIR}/${BIN_NAME}"
	else
		need_cmd sudo
		sudo install -m 755 "$BINARY" "${INSTALL_DIR}/${BIN_NAME}"
	fi
fi

info "DevScope instalado em ${INSTALL_DIR}/${BIN_NAME} ✓"

# Adiciona ao PATH automaticamente se necessário
add_to_path() {
	local dir="$1"
	local profile_file="$2"

	if [[ -f "$profile_file" ]]; then
		if ! grep -q "$dir" "$profile_file" 2>/dev/null; then
			echo "" >> "$profile_file"
			echo "# DevScope" >> "$profile_file"
			echo "export PATH=\"${dir}:\$PATH\"" >> "$profile_file"
			echo "  adicionado ao ${profile_file}"
		fi
	fi
}

if ! command -v devscope >/dev/null 2>&1; then
	echo ""
	echo "==> Adicionando ${INSTALL_DIR} ao seu PATH..."

	ADDED=0
	if [[ -n "${BASH_VERSION:-}" ]] || [[ -f "$HOME/.bashrc" ]]; then
		add_to_path "$INSTALL_DIR" "$HOME/.bashrc"
		ADDED=1
	fi
	if [[ -n "${ZSH_VERSION:-}" ]] || [[ -f "$HOME/.zshrc" ]]; then
		add_to_path "$INSTALL_DIR" "$HOME/.zshrc"
		ADDED=1
	fi
	if [[ $ADDED -eq 0 ]]; then
		add_to_path "$INSTALL_DIR" "$HOME/.profile"
	fi

	echo ""
	echo "  Para aplicar agora, execute:"
	echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
	echo "  Ou abra um novo terminal."
fi

echo ""
echo "  ✅ DevScope ${TAG} instalado com sucesso!"
echo "  Execute: devscope"
echo ""
