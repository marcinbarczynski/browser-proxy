# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.4] - 2026-05-06

v1.0.3 introduced a regression: content scripts declared in the
manifest are only auto-injected on FUTURE navigations, not into pages
that were already open at the time of extension load/reload. Result:
after upgrading from v1.0.2, link clicks on any tab the user already
had open passed through to Chrome unchanged. Looked like the
extension wasn't intercepting at all.

### Fixed

- **Programmatic injection on install/update/startup.** The SW now
  uses `chrome.scripting.executeScript` to inject `content_script.js`
  into every existing http(s) tab immediately after the extension
  loads. Previously-open pages now intercept clicks without needing
  a manual reload.
- **Idempotency guard** in `content_script.js`: a window-level flag
  prevents the click listeners from being registered twice when both
  the manifest auto-injection and the programmatic injection target
  the same tab.

### Added

- **Verbose console logging** with `[Browser Proxy CS]` (content
  script, in the page's DevTools) and `[Browser Proxy SW]` (service
  worker, in chrome://extensions → service worker → inspect). Logs
  cover: content script load, click intercepts, skipped clicks and
  why, handshake result, host response, passthrough vs redirect
  decision and the reason. Open both consoles when debugging.
- New `scripting` permission required for `chrome.scripting.executeScript`.

### Notes

- After upgrading: `chrome://extensions` → reload Browser Proxy. The
  SW will inject the content script into all your already-open
  http(s) tabs automatically. No need to manually F5 each one.

## [1.0.3] - 2026-05-06

Architecture rewrite of the Chrome extension. The webNavigation-based
intercept used by v1.0.0–v1.0.2 was unfixably racy: by the time
`onCommitted` fires (the only event that exposes `transitionType`),
Chrome has already allocated a renderer for the destination URL, the
DOM may have started constructing, and page JS may already have fired
a follow-up navigation that we never get to intercept — leaking tabs
into Chrome that the user wanted in another browser.

v1.0.3 moves interception to the `<a>` click event itself via a content
script. `preventDefault()` runs synchronously in the capture phase,
stopping Chrome from doing *anything* with the link before the host has
weighed in. There is no destination tab to flicker, no goBack to
race, no leakage.

### Changed

- **Interception moved from `webNavigation` to a content script.**
  Listens on `click` + `auxclick` in the capture phase, filters to
  `<a href>` with an http(s) destination, calls `preventDefault()`
  synchronously, then asks the background SW to make the routing call.
- **Modifier keys preserved on passthrough.** When the host says the
  URL stays in this browser, the background SW re-performs the
  navigation matching the original click's intent: `shift` → new
  window, `ctrl`/`cmd`/middle-click/`target=_blank` → new tab,
  otherwise same tab.
- **Address-bar input, bookmarks, reloads, form submits, history
  navigation, and JS-driven navigation pass through automatically**
  because they don't go through `<a>` click events. The v1.0.2
  `transitionType === "link"` filter is no longer needed and was
  removed along with its `webNavigation` listeners.

### Fixed

- Tab cascade when clicking a link routed to another browser. v1.0.2
  could leak two or three Chrome tabs per click when page JS fired a
  secondary navigation during the host-roundtrip window between
  `onCommitted` and `tabs.remove`.
- Same-document anchor jumps (`<a href="#section">`) are no longer
  routed.
- `alt`-click is no longer intercepted (it's the OS-level "save link"
  modifier on most platforms).

### Notes

- New manifest permission profile: `webNavigation` and `storage` are no
  longer requested. `nativeMessaging`, `tabs`, and `host_permissions`
  for http(s) remain (the latter is what allows the content script to
  inject everywhere).
- Slightly higher per-click latency: ~50–100 ms while we wait for the
  native host roundtrip. The host is one-shot per call; a future
  release may switch to `connectNative` to keep it warm.
- Reload the extension in `chrome://extensions` after upgrading
  (`browser-proxy install` extracts the new files but Chrome doesn't
  auto-reload unpacked extensions).

## [1.0.2] - 2026-05-06

### Changed

- **Only intercept link clicks**, not address-bar input. Previously the
  extension intercepted every top-level navigation, which meant typing
  `youtube.com` into Chrome's address bar (in profile A) would redirect
  the navigation to whatever the routing rule said (e.g. profile B) —
  the user explicitly chose this browser/profile by typing here, so
  hijacking it was wrong.

  Now only `transitionType === "link"` navigations are routed. Address
  bar (`typed`/`generated`), bookmarks (`auto_bookmark`), reloads,
  form submits, and back/forward all stay in the originating browser.

### Notes

- Implementation moved from `webNavigation.onBeforeNavigate` to
  `webNavigation.onCommitted` — the latter is the only event that
  exposes `transitionType`. The trade-off: by the time we tear down
  the navigation, the destination URL has already begun loading. In
  practice this is a brief flicker (or imperceptible on fast networks).
- New-tab detection (target=_blank, cmd-click, window.open) now uses
  `webNavigation.onCreatedNavigationTarget` instead of inspecting
  `tab.url` — the v1.0.1 heuristic ("is the tab on about:blank?")
  always reported false at onCommitted because tab.url is already the
  destination URL by then.

## [1.0.1] - 2026-05-06

Critical bug-fix release. v1.0.0 had a tab-cascade bug that could brick a
Chromium browser when the routing decision named a browser whose alias
wasn't in the extension's hardcoded `CURRENT_BROWSERS` list — the host
would "redirect" to that same browser the click came from, spawning a
fresh tab, firing `onBeforeNavigate` again, and repeating exponentially.

Triggered most easily by clicking the extension's toolbar icon: the
v1.0.0 popup pinged the host with a real URL and `current_browsers:
["__ping__"]`, which made the host treat the probe as a routable click
and open the URL in the configured default — kicking off the cascade
even if the user's setup was otherwise correct.

### Fixed

- **Popup health-check no longer routes**: `popup.js` now sends a
  dedicated `{ping: true}` message; the host short-circuits with
  `{ok: true}` without touching the routing pipeline or invoking
  `opener.Open`.
- **URL-keyed dedupe** in `background.js`: if a URL was just redirected,
  subsequent `onBeforeNavigate` events for the same URL are ignored for
  10 s — across tabs. v1.0.0's tabId-keyed cache could never match the
  cascade tabs since each new tab had a fresh id.
- **Global rate limit** in `background.js`: at most 10 host calls per
  3 s window. When exceeded, the extension fails open for 10 s. Bounds
  any pathological loop to ~1 s of work.
- **Ping handshake** in `background.js`: before sending any routable URL
  to the host, the extension confirms the host responds to `{ping}` with
  `{ok: true}`. A v1.0.0-era host that ignores ping is never invoked.

### Notes

- If you installed v1.0.0 and got hit by the cascade: stop Chrome, run
  `browser-proxy uninstall-extension <browser>` for every Chromium-family
  browser, upgrade to v1.0.1, then `browser-proxy install` again. The
  extension files extracted by v1.0.0 should be replaced by the v1.0.1
  ones automatically when `install` re-runs.

## [1.0.0] - 2026-05-06

First stable release. Adds in-browser routing via a companion Chrome
extension — clicks on links *inside* Chrome can now be redirected to a
different browser, not just clicks coming from other apps.

### Added

- **In-browser routing.** A Chrome MV3 companion extension intercepts
  top-level navigations via `webNavigation.onBeforeNavigate`, asks the
  daemon (over Native Messaging) whether the URL should land elsewhere,
  and cancels the in-Chrome navigation if so. Same TOML config is the
  single source of truth for both OS-level clicks and in-browser clicks.
- **`install` now sets up the extension end-to-end.** It extracts the
  bundled extension files into a per-OS data directory and registers the
  Native-Messaging host for every Chromium-family browser already present
  on the machine (Chrome / Beta / Canary, Chromium, Brave, Edge, Vivaldi,
  Arc, Opera). The user only has to load the extension via "Load unpacked".
- **Deterministic extension ID** — `lapppffemojdmedcoanjllncgiejbjjo`.
  Baked into `manifest.json` via the `key` field so the ID is identical
  on every machine; no per-machine ID copy-paste.
- New subcommand `install-extension <browser>` to register the host for
  browsers added after the initial install. Symmetric
  `uninstall-extension <browser>`.
- New subcommand `native-host` runs the stdio loop the extension talks
  to. Auto-invoked when the binary is launched with a
  `chrome-extension://<id>/` argv (Chrome's Native-Messaging convention)
  so the manifest's `path` can point straight at the binary — no wrapper
  script needed.
- `extension/` Go package embeds the extension assets (`manifest.json`,
  `background.js`, `popup.html`, `popup.js`) via `embed.FS`. A single
  binary contains everything required to install.
- `internal/nativehost/` package: Chrome Native-Messaging wire protocol
  (4-byte little-endian length prefix + JSON), with full unit tests.

### Changed

- `uninstall` now also removes the extracted extension files and every
  Native-Messaging manifest `install` registered.
- Extension files live at:
  - macOS: `~/Library/Application Support/browser-proxy/extension/chrome/`
  - Linux: `$XDG_DATA_HOME/browser-proxy/extension/chrome/`
    (default `~/.local/share/browser-proxy/extension/chrome/`)

### Notes

- `install` and `uninstall` deliberately leave
  `~/.config/browser-proxy/config.toml` alone. `init` is still the only
  command that writes that file, and it refuses to overwrite an existing
  one.
- Default-browser registration (macOS `.app` bundle, Linux `.desktop`
  file) is unchanged — only the post-install steps expanded.
- Firefox-flavoured extension is not in the box yet; same architecture
  works (`browser.runtime.connectNative`), separate manifest dialect.

## [0.9.0] - 2026-05-06

### Fixed

- macOS: app bundle is now visible in System Settings → Default web
  browser picker. The `Info.plist` was missing several Launch Services
  fields (`CFBundleTypeRole=Viewer`, `LSHandlerRank=Default`,
  `CFBundleDocumentTypes` for HTML, `NSPrincipalClass=NSApplication`,
  `NSAppleEventsUsageDescription`), without which Launch Services
  silently disqualified the bundle from the picker.

## [0.8.0] - 2026-05-06

### Added

- A bare `host = "example.com"` rule now also matches `www.example.com`.
  Explicit `host = "www.example.com"` keeps its old "exact match" behavior.

## [0.7.0] - 2026-05-06

### Added

- `browser-proxy profiles <browser>` lists the profile names of a
  Chromium- or Firefox-family browser, with both directory and display
  names so they can be used directly in a rule's `profile` field.

### Fixed

- Chromium channel path resolution: separator normalization + ordered
  channel matching so e.g. Chrome Beta no longer falls back to the
  stable-channel `Local State` JSON.

## [0.6.0] - 2026-05-06

### Added

- Built-in unwrappers for common redirect wrappers: Slack OIDC
  (`login_initiate_redirect`, JWT-decoded), Microsoft Safe Links
  (`*.safelinks.protection.outlook.com`), Google `/url`, LinkedIn
  `/redir`, Facebook `l.php`, YouTube `/redirect`. Routing rules now
  see the actual target URL, not the wrapper. Disable with
  `unwrap_redirects = false`.
- Recursive unwrapping (max 5 layers) for nested wrappers.

## [0.5.0] - 2026-05-06

Initial release. Cross-platform CLI that registers itself as the system
default browser on macOS and Linux and routes URLs to per-rule browsers
based on a TOML config.

### Added

- TOML routing rules with `prefix` / `suffix` / `regex` / `host`
  matchers, optional source-app matching, and per-rule
  browser/profile targeting.
- URL rewrites: force HTTPS, strip tracking parameters, host
  replacement.
- Optional file logging (off by default).
- `init`, `install`, `uninstall`, `open`, `test`, `daemon`, `config`,
  `version` subcommands.
- macOS: `.app` bundle with Apple Events listener
  (`NSAppleEventManager` / `kAEGetURL`).
- Linux: `.desktop` file with `xdg-mime` / `xdg-settings`
  registration.
- One-line curl install script.
- GitHub Actions release pipeline (Linux amd64/arm64,
  macOS amd64/arm64).

[1.0.4]: https://github.com/maxischmaxi/browser-proxy/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/maxischmaxi/browser-proxy/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/maxischmaxi/browser-proxy/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/maxischmaxi/browser-proxy/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/maxischmaxi/browser-proxy/releases/tag/v0.5.0
