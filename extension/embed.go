// Package extension exposes the browser-extension assets embedded into the
// binary so `browser-proxy install` can drop them onto disk without the
// source tree being present.
//
// ExtensionID is derived from the public RSA key baked into
// chrome/manifest.json's "key" field — it's the deterministic ID Chrome
// computes for our extension on every machine where it's loaded. Native-
// messaging manifests reference it in "allowed_origins".
package extension

import "embed"

// ExtensionID is the Chrome extension ID derived from the public key in
// chrome/manifest.json. Stable across installs because the key is fixed.
const ExtensionID = "lapppffemojdmedcoanjllncgiejbjjo"

//go:embed all:chrome
var Files embed.FS
