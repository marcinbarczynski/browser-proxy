# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/maxischmaxi/browser-proxy/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/maxischmaxi/browser-proxy/releases/tag/v0.5.0
