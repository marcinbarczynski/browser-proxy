// Content script — runs in every http(s) page at document_start.
//
// Intercepts <a href> clicks BEFORE Chrome processes them, so we can
// stop the navigation completely instead of fighting Chrome's
// webNavigation pipeline after the fact (the unfixably racy approach
// in v1.0.0–v1.0.2). preventDefault() in the click handler runs
// synchronously; Chrome doesn't allocate a renderer for the
// destination URL, no flicker, no race.
//
// Why both 'click' and 'auxclick': Chrome dispatches 'click' for the
// primary mouse button (button 0) and 'auxclick' for non-primary
// (1 = middle, 2 = right). Right-click is filtered out below.
//
// Idempotency guard: chrome.scripting.executeScript can re-inject
// this file into a tab where the manifest content_scripts already
// loaded it. Without the guard we'd register click listeners twice
// → two host calls per click → two firefox windows or rate-limit
// hits.

(() => {
  if (window.__browserProxyContentScriptLoaded) {
    return;
  }
  window.__browserProxyContentScriptLoaded = true;

  const LOG_PREFIX = "[Browser Proxy CS]";
  console.log(LOG_PREFIX, "loaded on", window.location.href);

  function shouldIntercept(e, a) {
    if (e.defaultPrevented) {
      console.log(LOG_PREFIX, "skip: defaultPrevented");
      return false;
    }
    if (e.button !== 0 && e.button !== 1) {
      return false;
    }
    if (e.altKey) {
      console.log(LOG_PREFIX, "skip: alt-click (download intent)");
      return false;
    }
    if (!a || !a.href) {
      return false;
    }
    if (!/^https?:\/\//i.test(a.href)) {
      console.log(LOG_PREFIX, "skip: non-http(s) href", a.href);
      return false;
    }
    // Same-document anchor jumps (#fragment within current page).
    try {
      const here = new URL(window.location.href);
      const there = new URL(a.href);
      if (
        here.origin === there.origin &&
        here.pathname === there.pathname &&
        here.search === there.search &&
        there.hash !== ""
      ) {
        console.log(LOG_PREFIX, "skip: same-document anchor");
        return false;
      }
    } catch {
      /* ignore parse errors */
    }
    return true;
  }

  function handleClick(e) {
    const a = e.target?.closest?.("a[href]");
    if (!a) {
      return;
    }
    if (!shouldIntercept(e, a)) {
      return;
    }

    const url = a.href;
    const newTab =
      a.target === "_blank" || e.metaKey || e.ctrlKey || e.button === 1;
    const newWindow = e.shiftKey;
    const background = e.button === 1;

    console.log(LOG_PREFIX, "intercept click", {
      url,
      newTab,
      newWindow,
      background,
    });

    e.preventDefault();
    e.stopPropagation();

    chrome.runtime
      .sendMessage({
        type: "route",
        url,
        newTab,
        newWindow,
        background,
        sourceUrl: window.location.href,
      })
      .then((resp) => {
        console.log(LOG_PREFIX, "background ack", resp);
      })
      .catch((err) => {
        console.warn(
          LOG_PREFIX,
          "sendMessage failed — click was preventDefault'd, navigation lost",
          err?.message ?? err
        );
      });
  }

  document.addEventListener("click", handleClick, true);
  document.addEventListener("auxclick", handleClick, true);
})();
