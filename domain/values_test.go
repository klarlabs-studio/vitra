package domain_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestNewAppID(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"com.example.myapp", false},
		{"", true},
		{"has space", true},
	}
	for _, tt := range tests {
		_, err := domain.NewAppID(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewAppID(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
	}
}

func TestNewWindowID(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"main", false},
		{"main-window", false},
		{"", true},
		{"1main", true},
		{"bad id", true},
	}
	for _, tt := range tests {
		_, err := domain.NewWindowID(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewWindowID(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
	}
}

func TestNewOrigin(t *testing.T) {
	ok, err := domain.NewOrigin("app://local")
	if err != nil || ok != domain.OriginPackagedLocal {
		t.Fatalf("packaged origin: got %q err=%v", ok, err)
	}
	if !ok.IsPackaged() {
		t.Fatal("expected packaged")
	}
	if _, err := domain.NewOrigin(""); err == nil {
		t.Fatal("expected empty origin error")
	}
	if _, err := domain.NewOrigin("not-a-url"); err == nil {
		t.Fatal("expected scheme error")
	}
	o, err := domain.NewOrigin("https://trusted.example")
	if err != nil {
		t.Fatal(err)
	}
	if o.Scheme() != "https" {
		t.Fatalf("scheme=%q", o.Scheme())
	}
}

func TestNewPermissionAndCommandNames(t *testing.T) {
	if _, err := domain.NewPermissionName("fs.read"); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.NewPermissionName(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := domain.NewCommandName("project.open"); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.NewGrantName("project-files"); err != nil {
		t.Fatal(err)
	}
}
