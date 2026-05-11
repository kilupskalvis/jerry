#!/bin/sh
set -e

REPO="kilupskalvis/jerry"
BINARY="jerry"

main() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        arm64)   arch="arm64" ;;
        *)       echo "Unsupported architecture: $arch"; exit 1 ;;
    esac

    case "$os" in
        linux|darwin) ;;
        *) echo "Unsupported OS: $os (use 'go install' on Windows)"; exit 1 ;;
    esac

    version=$(get_latest_version)
    if [ -z "$version" ]; then
        echo "Error: no releases found for $REPO"
        exit 1
    fi

    tarball="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${tarball}"
    checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    echo "Downloading jerry ${version} for ${os}/${arch}..."
    curl -sL "$url" -o "${tmpdir}/${tarball}"
    curl -sL "$checksums_url" -o "${tmpdir}/checksums.txt"

    expected=$(grep "$tarball" "${tmpdir}/checksums.txt" | awk '{print $1}')
    if [ -n "$expected" ]; then
        actual=$(sha256sum "${tmpdir}/${tarball}" 2>/dev/null || shasum -a 256 "${tmpdir}/${tarball}" | awk '{print $1}')
        actual=$(echo "$actual" | awk '{print $1}')
        if [ "$expected" != "$actual" ]; then
            echo "Checksum verification failed"
            echo "  expected: $expected"
            echo "  got:      $actual"
            exit 1
        fi
    fi

    tar -xzf "${tmpdir}/${tarball}" -C "$tmpdir"

    install_binary "${tmpdir}/${BINARY}"
}

get_latest_version() {
    curl -sI "https://github.com/${REPO}/releases/latest" \
        | grep -i "^location:" \
        | sed 's/.*tag\///' \
        | tr -d '\r\n'
}

install_binary() {
    src="$1"

    if try_install "$src" "/usr/local/bin"; then
        return
    fi

    if command -v sudo >/dev/null 2>&1; then
        if sudo install -m 755 "$src" "/usr/local/bin/${BINARY}" 2>/dev/null; then
            echo "Installed to /usr/local/bin/${BINARY} (via sudo)"
            verify_version
            return
        fi
    fi

    mkdir -p "${HOME}/.local/bin"
    install -m 755 "$src" "${HOME}/.local/bin/${BINARY}"
    echo "Installed to ~/.local/bin/${BINARY}"

    case ":$PATH:" in
        *":${HOME}/.local/bin:"*) ;;
        *) echo "Warning: ~/.local/bin is not on your PATH. Add it with:"
           echo "  export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
    esac

    verify_version
}

try_install() {
    src="$1"
    dir="$2"

    if [ -w "$dir" ]; then
        install -m 755 "$src" "${dir}/${BINARY}"
        echo "Installed to ${dir}/${BINARY}"
        verify_version
        return 0
    fi
    return 1
}

verify_version() {
    if command -v "$BINARY" >/dev/null 2>&1; then
        echo "$(${BINARY} --version)"
    fi
}

main
