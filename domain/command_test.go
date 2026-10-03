package domain_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestCommandDefinition_RequiresPermission(t *testing.T) {
	_, err := domain.NewCommandDefinition("x", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	cmd, err := domain.NewCommandDefinition("project.open", "Open", "fs.read")
	if err != nil {
		t.Fatal(err)
	}
	cmd.WithPlugin("fs")
	if cmd.Plugin() != "fs" || cmd.Permission() != "fs.read" {
		t.Fatalf("bad cmd %+v", cmd)
	}
}
