// Package bridge contains the injected frontend runtime for Vitra WebViews.
package bridge

// PreloadJS is injected at document start. Identity is never trusted from JS —
// the host stamps window/origin when handling invokes. Invoke traffic uses the
// versioned ipc envelope (protocol "1"); replies keep the compact bridge shape
// {id,ok,result|error} for __recv.
const PreloadJS = `
(function() {
  if (window.__vitra) return;
  const pending = new Map();
  const listeners = new Map();
  let seq = 0;
  function post(obj) {
    const payload = JSON.stringify(obj);
    if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
      window.chrome.webview.postMessage(payload);
      return;
    }
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
          protocol: "1",
          kind: "invoke",
          id: id,
          payload: {
            command: command,
            input: input === undefined ? null : input,
            resource_path: resourcePath || ""
          }
        });
      });
    },
    on: function(event, handler) {
      if (typeof event !== "string" || typeof handler !== "function") {
        throw new Error("vitra.on requires event name and handler");
      }
      let list = listeners.get(event);
      if (!list) { list = []; listeners.set(event, list); }
      list.push(handler);
      return function off() {
        const cur = listeners.get(event) || [];
        listeners.set(event, cur.filter(function(h) { return h !== handler; }));
      };
    },
    __recv: function(raw) {
      let msg;
      try { msg = typeof raw === "string" ? JSON.parse(raw) : raw; } catch (e) { return; }
      if (msg && msg.type === "event") {
        const hs = listeners.get(msg.event) || [];
        for (let i = 0; i < hs.length; i++) {
          try { hs[i](msg.payload); } catch (e) {}
        }
        return;
      }
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
