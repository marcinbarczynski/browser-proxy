// Browser Proxy — Chrome MV3 service worker.
//
// In v1.0.3 we moved click interception from webNavigation to a
// content script that calls preventDefault synchronously in the
// capture phase. The SW's job here:
//
//   1. On extension install/update/reload — programmatically inject
//      the content script into all already-open http(s) tabs (the
//      manifest content_scripts entry only auto-injects on FUTURE
//      navigations, not pages already loaded). Without this step,
//      the extension appears not to work until each tab is reloaded.
//   2. Confirm the native host speaks our protocol via {ping}/{ok}.
//   3. Apply a global rate limit.
//   4. On a "route" message from the content script: ask the host;
//      if redirect=true the host already opened externally and we do
//      nothing. Otherwise re-perform the navigation in Chrome to
//      match the original click's intent.
//
// All of step 1, 3, and the handler decisions log with [Browser Proxy SW]
// — open the SW's DevTools (chrome://extensions → "service worker" →
// inspect) to follow the flow.

const LOG_PREFIX = "[Browser Proxy SW]";
const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

const CURRENT_BROWSERS = [
  "Google Chrome",
  "Chrome",
  "google-chrome",
  "google-chrome-stable",
];

// ── Inject content script into existing tabs on install/reload ──

async function injectIntoExistingTabs() {
  let tabs;
  try {
    tabs = await chrome.tabs.query({ url: ["http://*/*", "https://*/*"] });
  } catch (err) {
    console.warn(LOG_PREFIX, "tabs.query failed:", err?.message ?? err);
    return;
  }
  let injected = 0;
  let skipped = 0;
  for (const tab of tabs) {
    try {
      await chrome.scripting.executeScript({
        target: { tabId: tab.id, allFrames: false },
        files: ["content_script.js"],
      });
      injected++;
    } catch (err) {
      // Tabs in restricted origins (chrome://, devtools://, the Chrome
      // Web Store) refuse executeScript. Silent skip.
      skipped++;
    }
  }
  console.log(
    LOG_PREFIX,
    `injected content script into ${injected} existing tab(s); skipped ${skipped}`
  );
}

chrome.runtime.onInstalled.addListener((details) => {
  console.log(LOG_PREFIX, "onInstalled:", details.reason);
  injectIntoExistingTabs();
});

chrome.runtime.onStartup.addListener(() => {
  console.log(LOG_PREFIX, "onStartup");
  injectIntoExistingTabs();
});

// ── Rate limit ──

const callTimestamps = [];
const MAX_CALLS_PER_WINDOW = 20;
const RATE_WINDOW_MS = 3_000;
const RATE_COOLDOWN_MS = 10_000;
let cooldownUntil = 0;

function rateLimitOK() {
  const now = Date.now();
  if (now < cooldownUntil) {
    return false;
  }
  while (callTimestamps.length && callTimestamps[0] < now - RATE_WINDOW_MS) {
    callTimestamps.shift();
  }
  if (callTimestamps.length >= MAX_CALLS_PER_WINDOW) {
    cooldownUntil = now + RATE_COOLDOWN_MS;
    console.warn(
      LOG_PREFIX,
      `rate limit hit (${MAX_CALLS_PER_WINDOW} clicks in ${RATE_WINDOW_MS}ms) — failing open for ${RATE_COOLDOWN_MS}ms`
    );
    return false;
  }
  callTimestamps.push(now);
  return true;
}

// ── Ping handshake ──

let pingPromise = null;

async function confirmHostHandshake() {
  if (pingPromise) {
    return pingPromise;
  }
  pingPromise = (async () => {
    try {
      const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
        ping: true,
      });
      const ok = resp && resp.ok === true;
      console.log(LOG_PREFIX, "handshake ok =", ok, "resp =", resp);
      return ok;
    } catch (err) {
      console.warn(LOG_PREFIX, "handshake failed:", err?.message ?? err);
      return false;
    }
  })();
  setTimeout(() => {
    pingPromise = null;
  }, 60_000);
  return pingPromise;
}

// ── Route message from content script ──

chrome.runtime.onMessage.addListener((msg, sender, _sendResponse) => {
  if (msg && msg.type === "route") {
    handleRoute(msg, sender);
  }
  return false; // fire-and-forget
});

async function handleRoute(msg, sender) {
  console.log(LOG_PREFIX, "route request:", msg.url, {
    newTab: msg.newTab,
    newWindow: msg.newWindow,
    background: msg.background,
  });

  let routedExternally = false;
  let reason = "no host call";

  if (!rateLimitOK()) {
    reason = "rate-limited";
  } else if (!(await confirmHostHandshake())) {
    reason = "handshake failed";
  } else {
    try {
      const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
        url: msg.url,
        current_browsers: CURRENT_BROWSERS,
      });
      console.log(LOG_PREFIX, "host response:", resp);
      routedExternally = !!(resp && resp.redirect);
      reason = routedExternally
        ? `redirect → ${resp.browser}${resp.profile ? ` [${resp.profile}]` : ""}`
        : "host says passthrough";
    } catch (err) {
      reason = "native messaging error: " + (err?.message ?? err);
      console.warn(LOG_PREFIX, reason);
    }
  }

  if (routedExternally) {
    console.log(LOG_PREFIX, "decision: routed externally —", reason);
    return;
  }

  console.log(LOG_PREFIX, "decision: passthrough —", reason);

  try {
    if (msg.newWindow) {
      await chrome.windows.create({ url: msg.url });
    } else if (msg.newTab) {
      await chrome.tabs.create({ url: msg.url, active: !msg.background });
    } else if (sender?.tab?.id !== undefined) {
      await chrome.tabs.update(sender.tab.id, { url: msg.url });
    } else {
      await chrome.tabs.create({ url: msg.url });
    }
  } catch (err) {
    console.warn(LOG_PREFIX, "passthrough nav failed:", err?.message ?? err);
  }
}
