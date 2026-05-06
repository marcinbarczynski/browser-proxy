// Browser Proxy — Chrome MV3 service worker.
//
// Hooks every top-level navigation, asks the native-messaging host whether
// the URL should be routed to a different browser, and — if yes — cancels
// the in-Chrome navigation. The native host is the same `browser-proxy`
// binary that handles default-browser clicks; it has already opened the URL
// elsewhere by the time it answers, so all we have to do here is undo the
// in-Chrome side.
//
// Loop protection (lessons from v1.0.0):
//
// The v1.0.0 cascade happened when the host's routing decision named a
// browser whose alias wasn't in CURRENT_BROWSERS — the host would then
// "redirect" to that same browser the click came from, which spawns a
// fresh tab, fires onBeforeNavigate again, and repeats exponentially.
// We now have THREE independent defenses that any one of which is enough
// to stop the cascade:
//
//   1. URL-keyed dedupe: if a URL was just redirected, ignore subsequent
//      events for the same URL for RECENTLY_REDIRECTED_TTL_MS — regardless
//      of which tab they fire in. The v1.0.0 cache was tabId-keyed and
//      never matched cascade tabs (each new tab has a new id).
//   2. Global rate limit: at most MAX_CALLS_PER_WINDOW host calls per
//      RATE_WINDOW_MS. Once the limit is exceeded we fail open for
//      RATE_COOLDOWN_MS so even a pathological loop terminates within ~1s.
//   3. Ping handshake on first request: the host MUST acknowledge a {ping}
//      message before we send routable URLs. If a v1.0.0 host is somehow
//      still installed and ignores ping, we never call it.

const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

// What the native host should compare its routing decision against to know
// "the click came from me, don't bounce it back".
const CURRENT_BROWSERS = [
  "Google Chrome",
  "Chrome",
  "google-chrome",
  "google-chrome-stable",
];

// Defense 1: URL-keyed dedupe (cross-tab, with TTL).
const recentlyRedirected = new Map(); // url -> timestamp
const RECENTLY_REDIRECTED_TTL_MS = 10_000;

function wasRecentlyRedirected(url) {
  const ts = recentlyRedirected.get(url);
  if (ts === undefined) return false;
  if (Date.now() - ts > RECENTLY_REDIRECTED_TTL_MS) {
    recentlyRedirected.delete(url);
    return false;
  }
  return true;
}

function rememberRedirected(url) {
  recentlyRedirected.set(url, Date.now());
}

// Defense 2: global call-rate limiter.
const callTimestamps = [];
const MAX_CALLS_PER_WINDOW = 10;
const RATE_WINDOW_MS = 3_000;
const RATE_COOLDOWN_MS = 10_000;
let cooldownUntil = 0;

function rateLimitOK() {
  const now = Date.now();
  if (now < cooldownUntil) return false;
  while (callTimestamps.length && callTimestamps[0] < now - RATE_WINDOW_MS) {
    callTimestamps.shift();
  }
  if (callTimestamps.length >= MAX_CALLS_PER_WINDOW) {
    cooldownUntil = now + RATE_COOLDOWN_MS;
    console.warn(
      `[Browser Proxy] rate limit hit (${MAX_CALLS_PER_WINDOW} calls in ` +
        `${RATE_WINDOW_MS}ms) — failing open for ${RATE_COOLDOWN_MS}ms. ` +
        "This usually means the native host is mis-routing; check your " +
        "config.toml's `default` and rule browser names against the " +
        "actual binary names on this OS."
    );
    return false;
  }
  callTimestamps.push(now);
  return true;
}

// Defense 3: confirm with the host that it speaks the v1.0.1+ protocol
// before we ever ship it a real URL. Cached in chrome.storage.session.
let pingPromise = null;

async function confirmHostHandshake() {
  if (pingPromise) return pingPromise;
  pingPromise = (async () => {
    try {
      const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
        ping: true,
      });
      return resp && resp.ok === true;
    } catch {
      return false;
    }
  })();
  // Re-probe periodically so a freshly-installed host is picked up without
  // requiring a Chrome restart.
  setTimeout(() => {
    pingPromise = null;
  }, 60_000);
  return pingPromise;
}

// Tab IDs we're currently tearing down — guards against the close/goBack
// itself triggering another onBeforeNavigate event we'd react to.
const tearingDown = new Set();

chrome.webNavigation.onBeforeNavigate.addListener(async (details) => {
  if (details.frameId !== 0) return;
  if (!/^https?:\/\//i.test(details.url)) return;
  if (tearingDown.has(details.tabId)) return;

  // Defense 1
  if (wasRecentlyRedirected(details.url)) {
    return;
  }
  // Defense 2
  if (!rateLimitOK()) {
    return;
  }
  // Defense 3
  if (!(await confirmHostHandshake())) {
    return;
  }

  let resp;
  try {
    resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
      url: details.url,
      current_browsers: CURRENT_BROWSERS,
    });
  } catch (err) {
    console.warn("[Browser Proxy] native host error:", err?.message ?? err);
    return;
  }

  if (!resp || !resp.redirect) return;

  rememberRedirected(details.url);
  tearingDown.add(details.tabId);
  try {
    await tearDownNavigation(details.tabId);
  } catch (err) {
    console.warn("[Browser Proxy] tear-down failed:", err?.message ?? err);
  } finally {
    tearingDown.delete(details.tabId);
  }
});

// tearDownNavigation cancels the in-browser side of a navigation we just
// redirected. Two cases:
//
//   1. Fresh tab — Chrome opened a new tab to load the URL (e.g. cmd-click,
//      bookmark, link from another app). Closing it is the cleanest result.
//      Special case: if it's the only tab in the window, replace with the
//      newtab page so closing it doesn't quit the browser.
//   2. Existing tab — the user clicked a link on a page they were already
//      on. We want to keep them on the prior page; goBack() does that, with
//      about:blank as a fallback when there's nothing to go back to.
async function tearDownNavigation(tabId) {
  let tab;
  try {
    tab = await chrome.tabs.get(tabId);
  } catch {
    return;
  }

  const isFresh =
    !tab.url ||
    tab.url === "" ||
    tab.url === "about:blank" ||
    tab.url === "chrome://newtab/" ||
    tab.url === "edge://newtab/";

  if (isFresh) {
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

// Periodic prune so the dedupe map can't grow unbounded if the SW stays
// warm for a long session. The TTL check on read already keeps things
// correct; this just bounds memory.
setInterval(() => {
  const now = Date.now();
  for (const [k, ts] of recentlyRedirected) {
    if (now - ts > RECENTLY_REDIRECTED_TTL_MS) recentlyRedirected.delete(k);
  }
}, 60_000);
