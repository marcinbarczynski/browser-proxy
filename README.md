# browser-proxy

A tiny CLI that registers itself as your **system default browser** on macOS and
Linux and routes every URL to the browser of your choice based on rules in a
TOML config.

Open a Slack link → `browser-proxy` decides whether it goes to Chrome, Firefox
(or any other app) based on prefix / hostname / regex / suffix matching.

Inspired by [Finicky](https://github.com/johnste/finicky), but cross-platform
and statically declared in TOML rather than JS.

## Build

```sh
go build -o browser-proxy ./cmd/browser-proxy
```

macOS builds need a Mac (cgo + Cocoa). Linux builds run anywhere with Go ≥ 1.21.

## Use

```sh
# 1. write the example config
./browser-proxy init

# 2. edit ~/.config/browser-proxy/config.toml — see Config below

# 3. dry-run a URL through the rules
./browser-proxy test "https://github.com/foo/bar"

# 4. install as system default browser
./browser-proxy install
#    Linux: writes ~/.local/share/applications/browser-proxy.desktop
#           and points xdg-mime / xdg-settings at it.
#    macOS: builds ~/Applications/Browser Proxy.app (ad-hoc-signed),
#           then asks you to pick it in System Settings → Default web browser.
```

## Config

`~/.config/browser-proxy/config.toml`. First matching rule wins; if no rule
matches, `default` is used. Exactly one matcher per rule.

```toml
default = "Google Chrome"

[[rules]]
prefix = "https://github.com/"
browser = "Firefox"

[[rules]]
host = "*.atlassian.net"          # "*." also matches the apex (atlassian.net)
browser = "Firefox"

[[rules]]
regex = "^https://meet\\.google\\.com"
browser = "Google Chrome"

[[rules]]
suffix = ".pdf"                    # case-insensitive, tested against URL path
browser = "Preview"
```

`browser` can be:

- **macOS**: an app name (`"Firefox"`) or a bundle ID (`"com.google.Chrome"`).
- **Linux**: a binary name or absolute path (`"firefox"`, `/usr/bin/qutebrowser`),
  or a `.desktop` file name (`"firefox.desktop"`, launched via `gio launch`).

## Commands

| Command          | What it does                                                |
| ---------------- | ----------------------------------------------------------- |
| `init`           | Write the example config                                    |
| `install`        | Register as system default browser                          |
| `uninstall`      | Remove the bundle / desktop file                            |
| `open <url>`     | Route the URL (called by the OS, also useful for testing)   |
| `test <url>`     | Print the chosen browser without opening anything           |
| `daemon`         | macOS-internal: Apple-Event listener invoked from the bundle |
| `config`         | Show the active config path and contents                    |
| `version`        | Print version                                               |

## How it works

- **Linux** registers a `.desktop` file with `MimeType=x-scheme-handler/http;…`
  and is invoked once per click as `browser-proxy open <url>`.
- **macOS** ships an `.app` bundle whose `Info.plist` declares `CFBundleURLTypes`
  for `http`/`https`. The binary inside listens to `kInternetEventClass` /
  `kAEGetURL` Apple Events via `NSAppleEventManager` (cgo + Cocoa) and stays
  resident as a `LSUIElement` background app — no Dock icon.
