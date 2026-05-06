const NATIVE_HOST = "com.maxischmaxi.browser_proxy";

const status = document.getElementById("status");
const button = document.getElementById("ping");

async function ping() {
  status.textContent = "Pinging native host…";
  status.className = "status";
  try {
    const resp = await chrome.runtime.sendNativeMessage(NATIVE_HOST, {
      url: "https://browser-proxy.invalid/__ping__",
      current_browsers: ["__ping__"],
    });
    if (resp && resp.error) {
      status.textContent = "Host responded with error: " + resp.error;
      status.className = "status err";
      return;
    }
    status.textContent = "Native host reachable. Routing is active.";
    status.className = "status ok";
  } catch (err) {
    status.textContent =
      "Native host not reachable: " +
      (err?.message ?? String(err)) +
      " — run `browser-proxy install-extension chrome <id>` and reload the extension.";
    status.className = "status err";
  }
}

button.addEventListener("click", ping);
ping();
