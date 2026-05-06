const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

const status = document.getElementById("status");
const button = document.getElementById("ping");

async function ping() {
  status.textContent = "Pinging native host…";
  status.className = "status";
  try {
    // Send a dedicated {ping:true} probe so the host short-circuits this
    // without going through routing / opener.Open. Sending a real URL here
    // was the v1.0.0 bug that triggered the tab-cascade.
    const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
      ping: true,
    });
    if (!resp || !resp.ok) {
      status.textContent =
        "Host responded but didn't acknowledge the ping" +
        (resp && resp.error ? `: ${resp.error}` : "") +
        " — your daemon may be older than v1.0.1.";
      status.className = "status err";
      return;
    }
    status.textContent = "Native host reachable. Routing is active.";
    status.className = "status ok";
  } catch (err) {
    status.textContent =
      "Native host not reachable: " +
      (err?.message ?? String(err)) +
      " — run `browser-proxy install-extension <browser>` and reload the extension.";
    status.className = "status err";
  }
}

button.addEventListener("click", ping);
ping();
