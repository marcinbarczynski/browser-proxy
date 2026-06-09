# browser-proxy

A tiny CLI that registers itself as your **system default browser** on macOS and
Linux and routes every URL to the browser of your choice based on rules in a
TOML config.

Open a Slack link → `browser-proxy` decides whether it goes to Chrome, Firefox
(or any other app) based on prefix / hostname / regex / suffix matching.

Inspired by [Finicky](https://github.com/johnste/finicky), but cross-platform
and statically declared in TOML rather than JS.

## Install

`browser-proxy` runs on Linux and macOS. Pick whichever route fits your setup:

- [**Arch Linux (AUR)**](#arch-linux-aur) — `yay -S browser-proxy`
- [**Install script**](#install-script) — one-liner for any Linux/macOS
- [**Manual download**](#manual-download) — fetch a release binary yourself
- [**Build from source**](#build-from-source)

### Arch Linux (AUR)

Published to the AUR as [`browser-proxy`](https://aur.archlinux.org/packages/browser-proxy).
With an AUR helper:

```sh
yay -S browser-proxy        # or: paru -S browser-proxy
```

…or build it by hand:

```sh
git clone https://aur.archlinux.org/browser-proxy.git
cd browser-proxy
makepkg -si
```

The package builds from source (`makedepends=go`) and is bumped automatically
on every tagged release.

### Install script

One-line install of the latest release for your OS/arch:

```sh
curl -fsSL https://raw.githubusercontent.com/maxischmaxi/browser-proxy/main/install.sh | sh
```

The script picks `linux-amd64`, `linux-arm64`, `darwin-amd64` or `darwin-arm64`
automatically and drops the binary into `/usr/local/bin` (or `~/.local/bin` if
that's not writable). On macOS it strips the quarantine attribute so the
binary runs without a Gatekeeper dialog.

Pin a specific version or override the destination via env vars:

```sh
BROWSER_PROXY_VERSION=v0.5.0 \
BROWSER_PROXY_DEST=$HOME/bin \
  curl -fsSL https://raw.githubusercontent.com/maxischmaxi/browser-proxy/main/install.sh | sh
```

### Manual download

Download the asset for your platform from the
[releases page](https://github.com/maxischmaxi/browser-proxy/releases),
verify, and put it on your PATH:

```sh
# pick the right line for your machine
ASSET=browser-proxy-linux-amd64    # or linux-arm64 / darwin-amd64 / darwin-arm64
curl -fsSL -O "https://github.com/maxischmaxi/browser-proxy/releases/latest/download/${ASSET}"
curl -fsSL -O "https://github.com/maxischmaxi/browser-proxy/releases/latest/download/SHA256SUMS"
sha256sum -c SHA256SUMS --ignore-missing   # or `shasum -a 256 -c` on macOS
chmod +x "${ASSET}" && sudo mv "${ASSET}" /usr/local/bin/browser-proxy
```

## Build from source

```sh
go build -o browser-proxy ./cmd/browser-proxy
```

Requires Go ≥ 1.22. Linux builds run anywhere; macOS builds need a Mac
(cgo + Cocoa for the Apple-Event handler).

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
matches, `default` is used. Each rule must set at least one constraint:

- **One** URL matcher: `prefix` | `suffix` | `regex` | `host` (mutually exclusive)
- And/or `source`: the app that opened the link
- `browser` (required)
- Optional `profile` (Chromium-likes and Firefox only)

When both a URL matcher and `source` are set, both must match (AND).

```toml
default = "Google Chrome"

[[rules]]
prefix = "https://github.com/"
browser = "Firefox"

[[rules]]
host = "*.atlassian.net"          # "*." also matches the apex (atlassian.net)
browser = "Firefox"

[[rules]]
host = "example.com"              # also auto-matches www.example.com
browser = "Brave"

[[rules]]
regex = "^https://meet\\.google\\.com"
browser = "Google Chrome"

[[rules]]
suffix = ".pdf"                    # case-insensitive, tested against URL path
browser = "Preview"

# Every link clicked from Slack opens in Chrome — regardless of URL.
[[rules]]
source = "Slack"
browser = "Google Chrome"

# Same for Microsoft Teams.
[[rules]]
source = "Microsoft Teams"
browser = "Google Chrome"

# Combined: docs.* links FROM Mail go to Safari.
[[rules]]
prefix = "https://docs."
source = "Mail"
browser = "Safari"
```

### `host` matching semantics

| Pattern               | Matches                                                |
| --------------------- | ------------------------------------------------------ |
| `host = "example.com"`   | `example.com` **and** `www.example.com`             |
| `host = "*.example.com"` | `example.com` (apex) **and** every `*.example.com` subdomain (`www.`, `api.`, …) |
| `host = "www.example.com"` | only `www.example.com` — bare apex is **not** matched |

The bare-host → `www.` fallback is for convenience: nearly every host you'd
write a routing rule for has a `www.` variant, and it's surprising if a
literal-looking pattern misses it. Use the explicit `www.` form (or a regex)
if you really want apex/sub split.

### `browser` values

- **macOS**: an app name (`"Firefox"`) or a bundle ID (`"com.google.Chrome"`).
- **Linux**: a binary name or absolute path (`"firefox"`, `/usr/bin/qutebrowser`),
  or a `.desktop` file name (`"firefox.desktop"`, launched via `gio launch`).

### `source` values

The originating app. Matching is case-insensitive.

- **macOS**: app name (`"Slack"`, `"Microsoft Teams"`, `"Mail"`) **or** bundle
  ID (`"com.tinyspeck.slackmacgap"`; Teams: `"com.microsoft.teams2"` for new
  Teams, `"com.microsoft.teams"` for classic). The dot-bearing form is
  interpreted as a bundle ID; everything else as `localizedName`.
- **Linux**: process name (`comm`) of the first non-launcher ancestor of this
  binary. Bundle IDs have no meaning here. Detection works best for apps that
  invoke `xdg-open`/`gio launch` directly; sandboxed/portal-based launches may
  produce no source.

Use `browser-proxy test -source <name> <url>` to dry-run rules with a
simulated source app.

### `profile` values

Chrome and Firefox each store profiles differently — `browser-proxy` translates
the spec into the right launch flag automatically.

**Chromium family** (Chrome, Chromium, Brave, Edge, Vivaldi, Opera, Arc, Thorium, …)

Translates to `--profile-directory=<dir>`. The spec can be either:

- the **directory name** as Chrome stores it (`"Default"`, `"Profile 1"`, …), or
- the **display name** you chose in Chrome (`"Work"`, `"Personal"`, …) — looked
  up in the browser's `Local State` JSON and resolved to the directory name.

Display-name lookup is case-insensitive. If lookup fails (no `Local State`
found, or no match) the spec is passed through unchanged so non-standard
installs still work.

**Firefox family** (Firefox, LibreWolf, Waterfox, Tor Browser, Zen, …)

Translates to `-P "<name>" --new-instance`. The spec is the profile name shown
in `about:profiles` / Firefox's profile manager. `--new-instance` is added
automatically because `-P` is otherwise silently ignored when Firefox is
already running with another profile.

**Other browsers** (Safari, qutebrowser, …)

`profile` is ignored with a stderr warning. Safari profiles (Sonoma+) have no
public CLI flag.

**Caveat — `.desktop` files on Linux**

When `browser` ends in `.desktop`, launching goes through `gio launch`, which
can't forward extra flags. `profile` is dropped with a warning. Use a binary
name (`firefox`, `google-chrome`) if you need profile targeting.

## URL rewrites

Rewrites are applied to the incoming URL **before** routing — so rules can
match the rewritten form. Four layers run in order:
`force_https` → `unwrap_redirects` → `strip_params` → `[[rewrites]]`.

### Built-in unwrappers

Many apps don't hand you the link you actually clicked — they wrap it in their
own redirect (for SSO, scanning, tracking, …). With `unwrap_redirects = true`
(default) browser-proxy peels these layers off so your routing rules can match
on the real destination. Recognised wrappers:

| Source                   | Pattern                                              | Where it lives           |
| ------------------------ | ---------------------------------------------------- | ------------------------ |
| **Slack** "Sign in with Slack" | `slack.com/openid/connect/login_initiate_redirect?login_hint=<JWT>` | JWT payload `target_uri` claim |
| **Microsoft Safe Links** (Outlook) | `*.safelinks.protection.outlook.com/?url=…` | `url` query param        |
| **Microsoft Teams** Safe Links interstitial | `statics.teams.cdn.office.net/evergreen-assets/safelinks/…/atp-safelinks.html?url=…` | `url` query param        |
| **Google**               | `www.google.com/url?q=…` (Gmail, Calendar, Search)   | `q` (or `url`) param     |
| **LinkedIn**             | `www.linkedin.com/redir/redirect?url=…`              | `url` param              |
| **Facebook**             | `l.facebook.com/l.php?u=…` (and `lm.facebook.com`)   | `u` param                |
| **YouTube**              | `www.youtube.com/redirect?q=…`                       | `q` param                |

Wrappers can be nested (e.g. an Outlook email containing a Slack OIDC link →
Microsoft wraps Slack wraps Atlassian); unwrapping recurses up to 5 layers.

For Slack OIDC the JWT signature is **not** verified — that's the next hop's
job. We only extract the `target_uri` claim. URL targets are scheme-checked
to be `http`/`https` so a malicious wrapper can't smuggle a `javascript:`
redirect through.

Disable with `unwrap_redirects = false`.

### Other layers

```toml
# Upgrade every http:// to https:// before matching the rules.
force_https = true

# Drop tracking parameters from every URL. Trailing "*" = prefix wildcard.
strip_params = ["utm_*", "fbclid", "gclid", "mc_cid", "ref"]

# Hostname swap (preserves port, supports "*." wildcard).
[[rewrites]]
host = "twitter.com"
replacement_host = "nitter.net"

[[rewrites]]
host = "*.youtube.com"
replacement_host = "invidious.example"

# Generic regex replace (Go re2 syntax; $1, $2 are backreferences).
[[rewrites]]
regex = "^https://www\\.google\\.com/search\\?q=(.+)$"
replacement = "https://duckduckgo.com/?q=$1"
```

Each `[[rewrites]]` entry must use **either** `host` + `replacement_host`
**or** `regex` + `replacement`, never both. Rules are applied in declaration
order — later rules see the output of earlier ones, so you can chain them.

`browser-proxy test <url>` prints the rewritten URL when it differs from the
input, so you can debug rule chains without opening a browser.

## Logging

Off by default. Turn on with `log = true` in the config; every `open` call then
appends one or more lines to a per-user log file:

```
2026-05-05 14:23:45  rewrite  http://github.com/foo?utm_source=x  →  https://github.com/foo
2026-05-05 14:23:45  routed   https://github.com/foo  →  Firefox  (rule 0: prefix=https://github.com/)  source=Slack
2026-05-05 14:23:46  routed   https://nitter.net/x  →  Google Chrome [profile=Personal]  (default)
2026-05-05 14:23:47  error    open "Firefox": exit status 1
```

```toml
log = true
# log_file = "~/browser-proxy.log"   # optional override
```

Defaults if `log_file` is unset:

- **macOS**: `~/Library/Logs/browser-proxy.log` (visible in Console.app)
- **Linux**: `$XDG_STATE_HOME/browser-proxy/browser-proxy.log`
  (typically `~/.local/state/browser-proxy/browser-proxy.log`)

The file is created on demand with mode `0600` (URL history is sensitive) and
appended to. The `test` dry-run command never touches the log file. If the
file can't be opened, browser-proxy prints a one-line warning to stderr and
keeps routing — logging is best-effort and never blocks the click.

## Commands

| Command              | What it does                                                 |
| -------------------- | ------------------------------------------------------------ |
| `init`               | Write the example config                                     |
| `install`            | Register as system default browser                           |
| `uninstall`          | Remove the bundle / desktop file                             |
| `open <url>`         | Route the URL (called by the OS, also useful for testing)    |
| `test <url>`         | Print the chosen browser without opening anything            |
| `profiles <browser>` | List a Chromium- or Firefox-family browser's profile names   |
| `daemon`             | macOS-internal: Apple-Event listener invoked from the bundle |
| `config`             | Show the active config path and contents                     |
| `version`            | Print version                                                |

### Discovering profile names

Run `browser-proxy profiles <browser>` to see what to put into a rule's
`profile` field. Works with both the binary name and the macOS app/bundle ID:

```
$ browser-proxy profiles google-chrome-beta
Browser: google-chrome-beta (Chromium family)
Source:  /home/max/.config/google-chrome-beta/Local State

DIRECTORY  DISPLAY NAME   EMAIL
Default    Personal       me@personal.com
Profile 1  Work           me@work.com

In a rule's 'profile' field you may use either form:
  profile = "Work"        # display-name lookup
  profile = "Profile 1"   # direct directory match
```

Firefox shows the names you'd pass to `firefox -P`, plus which one starts by
default (marked `*`). LibreWolf and Waterfox use their own profile dirs and
work the same.

## Scope: OS-level routing only

`browser-proxy` only sees URLs that come **from other apps** (Slack, Mail,
Terminal, system notifications, …) — i.e. anything the OS dispatches via
its default-browser machinery (`xdg-open` / Apple Events). Clicks **inside**
a browser are handled internally by that browser and never reach us. An
in-browser companion-extension shipped briefly in v1.0.x and was rolled
back in v1.1.0 because the click-intercept flow turned out unreliable.

If you need cross-profile / cross-browser routing for clicks that already
happen inside a browser, the practical options today are:
- a per-browser extension you install yourself (e.g. *Open in Browser*-style)
- a profile-switching keystroke (Chrome's "switch profile" / "guest"); or
- moving the link source out of the browser (e.g. configure your work apps
  to deep-link via `xdg-open`, which goes through us).

## How it works

- **Linux** registers a `.desktop` file with `MimeType=x-scheme-handler/http;…`
  and is invoked once per click as `browser-proxy open <url>`.
- **macOS** ships an `.app` bundle whose `Info.plist` declares `CFBundleURLTypes`
  for `http`/`https`. The binary inside listens to `kInternetEventClass` /
  `kAEGetURL` Apple Events via `NSAppleEventManager` (cgo + Cocoa) and stays
  resident as a `LSUIElement` background app — no Dock icon.

## License

[MIT](LICENSE).
