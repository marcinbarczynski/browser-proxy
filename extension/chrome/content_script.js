// Content script — runs in every http(s) page at document_start.
//
// Intercepts <a href> clicks BEFORE Chrome processes them, so we can stop
// the navigation completely instead of fighting Chrome's webNavigation
// pipeline after the fact. This was the architecture switch in v1.0.3
// after v1.0.2's onCommitted-based approach turned out to be unfixably
// racy (page JS could fire window.open or window.location.href = ...
// during the host-roundtrip window, leaking navigations into Chrome that
// we never got to intercept).
//
// We listen in the capture phase so page-level handlers don't beat us
// to it. If the page already preventDefault'd the click (e.g. the page
// runs its own client-side router), we leave it alone.
//
// Why both 'click' and 'auxclick': Chrome dispatches 'click' for the
// primary button (button 0) and 'auxclick' for non-primary (button 1
// = middle, button 2 = right). Right-click is filtered out below.
//
// preventDefault() in the click handler stops:
//   - same-tab navigation
//   - new-tab opening from middle-click / cmd-click / target=_blank
//
// After preventDefault we hand off to the background script via
// chrome.runtime.sendMessage. The background does the host roundtrip,
// then either:
//   - leaves Chrome unchanged (host already opened in the target browser)
//   - re-performs the navigation in Chrome (passthrough): tabs.update for
//     same-tab, tabs.create for new-tab, windows.create for new-window.

(() => {
  function shouldIntercept(e, a) {
    if (e.defaultPrevented) return false;
    // Right-click (button 2) → never intercept; that's the context menu.
    if (e.button !== 0 && e.button !== 1) return false;
    // alt-click is a download intent on most platforms — leave it alone.
    if (e.altKey) return false;
    // Need an http(s) destination. <a href="#section">, mailto:, javascript:,
    // tel:, etc. all skip.
    if (!a || !a.href) return false;
    if (!/^https?:\/\//i.test(a.href)) return false;
    // Same-document anchor jumps (#fragment within current page) shouldn't
    // route — they're not real navigations.
    try {
      const here = new URL(window.location.href);
      const there = new URL(a.href);
      if (
        here.origin === there.origin &&
        here.pathname === there.pathname &&
        here.search === there.search &&
        there.hash !== ""
      ) {
        return false;
      }
    } catch {
      /* ignore parse errors */
    }
    return true;
  }

  function handleClick(e) {
    const a = e.target?.closest?.("a[href]");
    if (!a) return;
    if (!shouldIntercept(e, a)) return;

    const url = a.href;
    const newTab =
      a.target === "_blank" || e.metaKey || e.ctrlKey || e.button === 1;
    const newWindow = e.shiftKey;
    // Middle-click is conventionally a background tab on Chrome/Firefox.
    const background = e.button === 1;

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
      .catch((err) => {
        // Background SW might be cold; surface in console. The user's click
        // will be lost in this case — rare, only if the SW is broken.
        console.warn("[Browser Proxy] sendMessage failed:", err?.message ?? err);
      });
  }

  document.addEventListener("click", handleClick, true);
  document.addEventListener("auxclick", handleClick, true);
})();
