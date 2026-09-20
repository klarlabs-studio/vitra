package bridge_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/vitra/bridge"
)

func TestPreloadJS_DefinesSecureInvokeSurface(t *testing.T) {
	js := bridge.PreloadJS
	for _, want := range []string{"window.__vitra", "invoke", "__recv", "webkit.messageHandlers.vitra"} {
		if !strings.Contains(js, want) {
			t.Fatalf("preload missing %q", want)
		}
	}
}
