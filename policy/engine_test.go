package policy_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
)

func TestEngine_ProductionForcesSignatureAndBlocksDevPerms(t *testing.T) {
	// Security invariants 9 (updates) and 11 (dev privileges).
	eng, err := policy.NewEngine(policy.Document{
		DenyPermissions:       []domain.PermissionName{"shell.exec"},
		AllowedUpdateChannels: []updater.Channel{updater.ChannelStable},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	if !eng.RequireUpdateSignature() {
		t.Fatal("production must require signatures")
	}
	if eng.AllowsPermission("dev.hot_reload") {
		t.Fatal("dev privilege must not leak into production")
	}
	if eng.AllowsPermission("shell.exec") {
		t.Fatal("enterprise deny list")
	}
	if eng.AllowsUpdateChannel(updater.ChannelBeta) {
		t.Fatal("beta channel should be blocked")
	}
	d := eng.OverlayDecision("shell.exec", domain.Decision{Allowed: true, Reason: "app grant"})
	if d.Allowed {
		t.Fatalf("expected overlay deny: %+v", d)
	}
}

func TestEngine_AuthorizeUpdate(t *testing.T) {
	eng, err := policy.NewEngine(policy.Document{
		AllowedUpdateChannels: []updater.Channel{updater.ChannelStable},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	ok := updater.Manifest{Channel: updater.ChannelStable, Signature: "deadbeef"}
	if err := eng.AuthorizeUpdate(ok); err != nil {
		t.Fatal(err)
	}
	if err := eng.AuthorizeUpdate(updater.Manifest{Channel: updater.ChannelBeta, Signature: "x"}); err == nil {
		t.Fatal("expected beta channel deny")
	}
	if err := eng.AuthorizeUpdate(updater.Manifest{Channel: updater.ChannelStable}); err == nil {
		t.Fatal("expected unsigned deny in production")
	}
}
