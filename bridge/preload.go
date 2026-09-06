// Package bridge contains the injected frontend runtime for Vitra WebViews.
package bridge

// PreloadJS is injected at document start. Identity is never trusted from JS —
// the host stamps window/origin when handling invokes.
const PreloadJS = `
(function() {
  if (window.__vitra) return;
  const pending = new Map();
  let seq = 0;
  function post(obj) {
    const payload = JSON.stringify(obj);
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.vitra) {
      window.webkit.messageHandlers.vitra.postMessage(payload);
      return;
    }
    throw new Error("vitra native bridge unavailable");
  }
  window.__vitra = {
    invoke: function(command, input, resourcePath) {
      return new Promise(function(resolve, reject) {
        const id = "c" + (++seq);
        pending.set(id, { resolve: resolve, reject: reject });
        post({
          type: "invoke",
          id: id,
          command: command,
          input: input === undefined ? null : input,
          resource_path: resourcePath || ""
        });
      });
    },
    __recv: function(raw) {
      let msg;
      try { msg = typeof raw === "string" ? JSON.parse(raw) : raw; } catch (e) { return; }
      const p = pending.get(msg.id);
      if (!p) return;
      pending.delete(msg.id);
      if (msg.ok) p.resolve(msg.result);
      else p.reject(Object.assign(new Error(msg.error || "denied"), { code: msg.code || "error" }));
    }
  };
  window.vitra = window.__vitra;
})();
`
