package bridge_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/vitra/bridge"
)

func TestPreloadJS_DefinesSecureInvokeSurface(t *testing.T) {
	js := bridge.PreloadJS
	for _, want := range []string{"window.__vitra", "invoke", "protocol", "kind", "on:", "__recv", "type === \"event\"", "webkit.messageHandlers.vitra", "chrome.webview"} {
		if !strings.Contains(js, want) {
			t.Fatalf("preload missing %q", want)
		}
	}
}
