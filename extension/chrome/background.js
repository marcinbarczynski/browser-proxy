// Browser Proxy — Chrome MV3 service worker.
//
// In v1.0.3 the SW no longer listens to webNavigation. Click interception
// happens in content_script.js, which preventDefault's the click and
// forwards us a {type:"route", url, newTab, newWindow, background} message.
// Our job here:
//
//   1. Confirm the native host speaks our protocol (ping handshake).
//   2. Apply a global rate limit so a buggy site spamming clicks can't
//      DoS the host.
//   3. Forward the URL to the host. If the host says redirect=true it has
//      already opened the URL in the target browser — we do nothing more
//      and the in-Chrome side is naturally absent (the click was prevented).
//   4. If the host says redirect=false (or is unreachable), re-perform the
//      navigation in Chrome ourselves: tabs.update for same-tab,
//      tabs.create for new-tab, windows.create for new-window. The user
//      still gets where they wanted to go.

const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

// What the host should compare its routing decision against to recognise
// "this click came from me, don't bounce it back here".
const CURRENT_BROWSERS = [
  "Google Chrome",
  "Chrome",
  "google-chrome",
  "google-chrome-stable",
];

// Global rate limit. Click interception is one click → one host call,
// so legitimate use is bounded by user typing speed. This guards against
// a misbehaving page firing synthetic clicks.
const callTimestamps = [];
const MAX_CALLS_PER_WINDOW = 20;
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
      `[Browser Proxy] rate limit hit (${MAX_CALLS_PER_WINDOW} clicks in ` +
        `${RATE_WINDOW_MS}ms) — failing open for ${RATE_COOLDOWN_MS}ms.`
    );
    return false;
  }
  callTimestamps.push(now);
  return true;
}

// Confirm the host responds to {ping:true} with {ok:true} before we ever
// send it a real URL. Cached for 60s; a freshly-installed v1.0.1+ host
// is picked up without requiring a Chrome restart.
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
  setTimeout(() => {
    pingPromise = null;
  }, 60_000);
  return pingPromise;
}

chrome.runtime.onMessage.addListener((msg, sender, _sendResponse) => {
  if (msg && msg.type === "route") {
    handleRoute(msg, sender);
  }
  return false; // we don't send a response back; content script is fire-and-forget
});

async function handleRoute(msg, sender) {
  let routedExternally = false;

  if (rateLimitOK() && (await confirmHostHandshake())) {
    try {
      const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
        url: msg.url,
        current_browsers: CURRENT_BROWSERS,
      });
      routedExternally = !!(resp && resp.redirect);
    } catch (err) {
      console.warn(
        "[Browser Proxy] native host error:",
        err?.message ?? err
      );
    }
  }

  if (routedExternally) {
    // Host already opened in the target browser. Do nothing in Chrome.
    return;
  }

  // Passthrough: re-perform the navigation as Chrome would have done it
  // before our content script preventDefault'd. Match the click's intent:
  // shift = new window, ctrl/cmd/middle/_blank = new tab, otherwise same
  // tab.
  try {
    if (msg.newWindow) {
      await chrome.windows.create({ url: msg.url });
    } else if (msg.newTab) {
      await chrome.tabs.create({ url: msg.url, active: !msg.background });
    } else if (sender?.tab?.id !== undefined) {
      await chrome.tabs.update(sender.tab.id, { url: msg.url });
    } else {
      // Sender tab info missing — fall back to a new tab so the click isn't
      // silently lost.
      await chrome.tabs.create({ url: msg.url });
    }
  } catch (err) {
    console.warn(
      "[Browser Proxy] passthrough nav failed:",
      err?.message ?? err
    );
  }
}
