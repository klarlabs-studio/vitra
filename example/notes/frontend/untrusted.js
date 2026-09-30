(function () {
  "use strict";
  const lines = [];
  const log = (s) => {
    lines.push(s);
    const p = document.createElement("p");
    p.textContent = s;
    document.body.append(p);
  };

  // 1. Use the top frame's bridge object.
  try {
    window.top.vitra.invoke("notes.read", {});
    log("top.vitra: reachable (unexpected)");
  } catch (e) {
    log("top.vitra: blocked (" + e.name + ")");
  }

  // 2. Post straight to the native message handler with a made-up token.
  const forged = JSON.stringify({
    protocol: "1", kind: "invoke", id: "forged", token: "0".repeat(64),
    payload: { command: "notes.read", input: { path: "/etc/hosts" }, resource_path: "/etc/hosts" },
  });
  let posted = false;
  try {
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.vitra) {
      window.webkit.messageHandlers.vitra.postMessage(forged);
      posted = true;
    } else if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
      window.chrome.webview.postMessage(forged);
      posted = true;
    }
  } catch (e) {
    log("post: threw " + e.name);
  }
  log(posted ? "native handler: forged call posted" : "native handler: not reachable from this frame");

  window.parent.postMessage({ type: "untrusted-report", posted: posted, lines: lines }, "*");
})();
