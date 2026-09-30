// Package bridge contains the injected frontend runtime for Vitra WebViews.
package bridge

import "strings"

// preloadJS is injected at document start; Preload fills in __VITRA_TOKEN__
// per window. Identity is never trusted from JS: the host stamps
// window/origin when handling invokes. Invoke traffic uses the versioned ipc
// envelope (protocol "1"); replies keep the compact bridge shape
// {id,ok,result|error} for __recv.
const preloadJS = `
(function() {
  // WebView2 injects document-created scripts into every frame; only the top
  // frame gets the bridge, and subframes never see the token.
  if (window.top !== window) return;
  if (window.__vitra) return;
  const token = "__VITRA_TOKEN__";
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
          token: token,
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

// Preload returns the bridge script for one window. token authenticates the
// window's top frame to the host: it lives only in the script's closure and is
// sent with every message, so a subframe (for example a cross-origin iframe
// that can reach the native message handler) cannot forge calls. token must be
// hex; anything else panics, since it is embedded in a JavaScript string.
func Preload(token string) string {
	if token == "" {
		panic("bridge: empty token")
	}
	for _, r := range token {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			panic("bridge: token must be hex")
		}
	}
	return strings.Replace(preloadJS, "__VITRA_TOKEN__", token, 1)
}
