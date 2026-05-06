// Browser Proxy — Chrome MV3 service worker.
//
// Hooks every top-level navigation, asks the native-messaging host whether
// the URL should be routed to a different browser, and — if yes — cancels
// the in-Chrome navigation. The native host is the same `browser-proxy`
// binary that handles default-browser clicks; it has already opened the URL
// elsewhere by the time it answers, so all we have to do here is undo the
// in-Chrome side.
//
// Native host name MUST match platform.NativeMessagingHostName in the Go
// code; the manifest under NativeMessagingHosts/ is keyed by it.

const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

// What the native host should compare its routing decision against to know
// "the click came from me, don't bounce it back". Order doesn't matter; the
// host does case-insensitive matching against any one of these.
const CURRENT_BROWSERS = [
  "Google Chrome",
  "Chrome",
  "google-chrome",
  "google-chrome-stable",
];

// Tab IDs we're currently tearing down — guards against the close/goBack
// itself triggering another onBeforeNavigate event we'd react to.
const tearingDown = new Set();

// Per-tab bookkeeping so we don't double-handle the same URL when Chrome
// fires the listener twice (it does, e.g. for committed-then-error transitions).
const recentlyHandled = new Map(); // key=tabId -> {url, ts}
const RECENT_TTL_MS = 5_000;

function rememberHandled(tabId, url) {
  recentlyHandled.set(tabId, { url, ts: Date.now() });
}

function wasJustHandled(tabId, url) {
  const entry = recentlyHandled.get(tabId);
  if (!entry) return false;
  if (Date.now() - entry.ts > RECENT_TTL_MS) {
    recentlyHandled.delete(tabId);
    return false;
  }
  return entry.url === url;
}

chrome.webNavigation.onBeforeNavigate.addListener(async (details) => {
  // Top-level only — iframes/subframes inherit the top-level decision.
  if (details.frameId !== 0) return;

  // Schemes other than http(s) (chrome://, file://, devtools://, …) are
  // never something we want to bounce.
  if (!/^https?:\/\//i.test(details.url)) return;

  if (tearingDown.has(details.tabId)) return;
  if (wasJustHandled(details.tabId, details.url)) return;

  let resp;
  try {
    resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
      url: details.url,
      current_browsers: CURRENT_BROWSERS,
    });
  } catch (err) {
    // Native host not installed, daemon crashed, manifest missing — fail
    // open: let the navigation proceed in-browser. We log to the SW console
    // so users can find it via chrome://serviceworker-internals.
    console.warn("[Browser Proxy] native host error:", err?.message ?? err);
    return;
  }

  if (!resp || !resp.redirect) return;

  rememberHandled(details.tabId, details.url);
  tearingDown.add(details.tabId);
  try {
    await tearDownNavigation(details.tabId);
  } catch (err) {
    console.warn("[Browser Proxy] tear-down failed:", err?.message ?? err);
  } finally {
    tearingDown.delete(details.tabId);
  }
});

// tearDownNavigation cancels the navigation we just intercepted. There are
// two cases:
//
//   1. Fresh tab — Chrome opened a new tab to load the URL (e.g. cmd-click,
//      bookmark, link from another app). Closing it is the cleanest result.
//   2. Existing tab — the user clicked a link in a page they were already
//      on. We want to keep them on the prior page; goBack() does that, with
//      about:blank as a fallback when there's nothing to go back to.
async function tearDownNavigation(tabId) {
  let tab;
  try {
    tab = await chrome.tabs.get(tabId);
  } catch {
    return; // tab already gone
  }

  const isFresh =
    !tab.url ||
    tab.url === "" ||
    tab.url === "about:blank" ||
    tab.url === "chrome://newtab/" ||
    tab.url === "edge://newtab/";

  if (isFresh) {
    // If this is the only tab in the window, closing it closes the window.
    // Replace with newtab instead in that case so Chrome doesn't quit.
    const tabs = await chrome.tabs.query({ windowId: tab.windowId });
    if (tabs.length <= 1) {
      await chrome.tabs.update(tabId, { url: "chrome://newtab/" });
      return;
    }
    await chrome.tabs.remove(tabId);
    return;
  }

  try {
    await chrome.tabs.goBack(tabId);
  } catch {
    await chrome.tabs.update(tabId, { url: "about:blank" });
  }
}

// Periodically prune stale recently-handled entries. The TTL check on read
// already keeps things correct, but this stops the map from growing
// unboundedly when the SW stays warm.
setInterval(() => {
  const now = Date.now();
  for (const [k, v] of recentlyHandled) {
    if (now - v.ts > RECENT_TTL_MS) recentlyHandled.delete(k);
  }
}, 60_000);
