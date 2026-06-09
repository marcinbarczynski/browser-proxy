#!/usr/bin/env sh
# browser-proxy installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/maxischmaxi/browser-proxy/main/install.sh | sh
#
# Uninstall the install-script copy again (also unregisters as default browser):
#   curl -fsSL https://raw.githubusercontent.com/maxischmaxi/browser-proxy/main/install.sh | sh -s -- uninstall
#
# Environment overrides:
#   BROWSER_PROXY_VERSION=v0.5.0   pin to a specific release (default: latest)
#   BROWSER_PROXY_DEST=/opt/bin    install dir override (default: /usr/local/bin if writable, else ~/.local/bin)
#   BROWSER_PROXY_UNINSTALL=1      same as passing the "uninstall" argument

set -eu

REPO="maxischmaxi/browser-proxy"

# ── Dispatch: install (default) vs uninstall ─────────────────────────────
ACTION="install"
case "${1:-}" in
  uninstall|--uninstall|remove) ACTION="uninstall" ;;
  install|"") ACTION="install" ;;
  *) echo "browser-proxy: unknown command: $1 (expected 'install' or 'uninstall')" >&2; exit 1 ;;
esac
[ "${BROWSER_PROXY_UNINSTALL:-}" = "1" ] && ACTION="uninstall"

# uninstall removes the binary that THIS script previously dropped on the
# system. It does not touch a package-manager-managed binary (e.g. an AUR
# install under /usr/bin) unless that path happens to be the one found.
uninstall() {
  # Same precedence as install, plus whatever is currently on PATH.
  candidates="${BROWSER_PROXY_DEST:-} /usr/local/bin ${HOME}/.local/bin"
  found=""
  for d in $candidates; do
    [ -n "$d" ] || continue
    [ -f "$d/browser-proxy" ] || continue
    case " $found " in *" $d/browser-proxy "*) ;; *) found="$found $d/browser-proxy" ;; esac
  done
  if command -v browser-proxy >/dev/null 2>&1; then
    onpath="$(command -v browser-proxy)"
    case " $found " in *" $onpath "*) ;; *) found="$found $onpath" ;; esac
  fi

  if [ -z "$found" ]; then
    echo "browser-proxy: nothing to uninstall — no binary found in" >&2
    echo "  ${BROWSER_PROXY_DEST:+$BROWSER_PROXY_DEST, }/usr/local/bin, ${HOME}/.local/bin or on PATH." >&2
    echo "  (If you installed via a package manager, remove it there, e.g. 'yay -R browser-proxy'.)" >&2
    exit 0
  fi

  # Unregister as the system default browser before deleting the binary.
  for bin in $found; do
    if [ -x "$bin" ]; then
      echo "browser-proxy: unregistering default-browser handler"
      "$bin" uninstall 2>/dev/null || true
      break
    fi
  done

  # Delete every copy we found; use sudo only when the dir isn't writable.
  for bin in $found; do
    dir="$(dirname "$bin")"
    echo "browser-proxy: removing $bin"
    if [ -w "$dir" ]; then
      rm -f "$bin"
    elif command -v sudo >/dev/null 2>&1; then
      sudo rm -f "$bin"
    else
      echo "browser-proxy: cannot remove $bin (no write permission and no sudo)" >&2
      echo "browser-proxy: remove it manually:  rm -f \"$bin\"" >&2
    fi
  done

  echo "browser-proxy: uninstalled. Your config at ~/.config/browser-proxy/config.toml was left untouched."
}

if [ "$ACTION" = "uninstall" ]; then
  uninstall
  exit 0
fi

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
