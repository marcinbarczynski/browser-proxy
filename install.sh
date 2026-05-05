#!/usr/bin/env sh
# browser-proxy installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/maxischmaxi/browser-proxy/main/install.sh | sh
#
# Environment overrides:
#   BROWSER_PROXY_VERSION=v0.5.0   pin to a specific release (default: latest)
#   BROWSER_PROXY_DEST=/opt/bin    install dir override (default: /usr/local/bin if writable, else ~/.local/bin)

set -eu

REPO="maxischmaxi/browser-proxy"

# ── Detect OS ────────────────────────────────────────────────────────────
case "$(uname -s)" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *) echo "browser-proxy: unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

# ── Detect architecture ──────────────────────────────────────────────────
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "browser-proxy: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# ── Build the download URL ───────────────────────────────────────────────
VERSION="${BROWSER_PROXY_VERSION:-latest}"
ASSET="browser-proxy-${OS}-${ARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

# ── Pick install location ────────────────────────────────────────────────
if [ -n "${BROWSER_PROXY_DEST:-}" ]; then
  DEST_DIR="$BROWSER_PROXY_DEST"
elif [ -w /usr/local/bin ]; then
  DEST_DIR="/usr/local/bin"
else
  DEST_DIR="$HOME/.local/bin"
fi
mkdir -p "$DEST_DIR"
DEST="$DEST_DIR/browser-proxy"

# ── Download to a temp file, then move atomically ────────────────────────
TMP="$(mktemp 2>/dev/null || mktemp -t browser-proxy)"
trap 'rm -f "$TMP"' EXIT INT TERM

echo "browser-proxy: downloading $URL"
if ! curl -fsSL "$URL" -o "$TMP"; then
  echo "browser-proxy: download failed (check that release '$VERSION' has asset '$ASSET')" >&2
  exit 1
fi
chmod +x "$TMP"

# macOS: clear the quarantine attribute so Gatekeeper doesn't block it.
# Silently skip if the binary wasn't quarantined (most curl downloads aren't).
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$TMP" 2>/dev/null || true
fi

mv "$TMP" "$DEST"
trap - EXIT INT TERM

# ── Done ─────────────────────────────────────────────────────────────────
INSTALLED_VERSION="$("$DEST" version 2>/dev/null || echo unknown)"
echo "browser-proxy: installed $INSTALLED_VERSION → $DEST"

# Warn if the install directory isn't on PATH.
case ":${PATH:-}:" in
  *":$DEST_DIR:"*) ;;
  *)
    echo "" >&2
    echo "browser-proxy: $DEST_DIR is not in your PATH." >&2
    case "${SHELL:-}" in
      *zsh*)  echo "  Add this to ~/.zshrc:  export PATH=\"$DEST_DIR:\$PATH\"" >&2 ;;
      *bash*) echo "  Add this to ~/.bashrc: export PATH=\"$DEST_DIR:\$PATH\"" >&2 ;;
      *)      echo "  Add: export PATH=\"$DEST_DIR:\$PATH\""                    >&2 ;;
    esac
    ;;
esac

echo ""
echo "Next steps:"
echo "  browser-proxy init       # write example config"
echo "  browser-proxy install    # register as system default browser"
