package notification_test

import (
	"testing"

	"go.klarlabs.de/vitra/plugin/official/notification"
)

func TestNotificationPlugin_Contribute(t *testing.T) {
	p := notification.New()
	m := p.Manifest()
	if m.ID != notification.PluginID {
		t.Fatalf("id: %s", m.ID)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "notifications.show" {
		t.Fatalf("perms: %v", m.Permissions)
	}
	c, err := p.Contribute()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) != 1 {
		t.Fatalf("commands: %d", len(c.Commands))
	}
	if c.Commands[0].Name() != "notifications.show" {
		t.Fatalf("command name: %s", c.Commands[0].Name())
	}
}
