package bridge_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/vitra/internal/bridge"
)

func TestPreload_DefinesSecureInvokeSurface(t *testing.T) {
	js := bridge.Preload("0123abcd")
	for _, want := range []string{"window.__vitra", "invoke", "protocol", "kind", "on:", "__recv", "type === \"event\"", "webkit.messageHandlers.vitra", "chrome.webview"} {
		if !strings.Contains(js, want) {
			t.Fatalf("preload missing %q", want)
		}
	}
}

// The token authenticates the top frame. It must be sent with every message,
// stay out of anything page scripts can reach, and never activate in a
// subframe (WebView2 injects document-created scripts into every frame).
func TestPreload_KeepsSenderTokenInTopFrameClosure(t *testing.T) {
	js := bridge.Preload("0123abcd")
	if !strings.Contains(js, `const token = "0123abcd";`) {
		t.Fatal("token not embedded as a closure constant")
	}
	if !strings.Contains(js, "token: token") {
		t.Fatal("token not sent with messages")
	}
	guard := strings.Index(js, "if (window.top !== window) return;")
	if guard < 0 || guard > strings.Index(js, "const token") {
		t.Fatal("preload must return in subframes before defining the token")
	}
	for _, leak := range []string{"window.__vitra.token", "__vitra.token", "window.token", "globalThis.token"} {
		if strings.Contains(js, leak) {
			t.Fatalf("token exposed via %s", leak)
		}
	}
}

func TestPreload_RejectsNonHexTokens(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for a token that could break out of the JS string")
		}
	}()
	bridge.Preload(`x"; alert(1); "`)
}
