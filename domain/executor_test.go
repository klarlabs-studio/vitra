package domain_test

import (
	"context"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestCommandExecutorFunc(t *testing.T) {
	fn := domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		return map[string]any{"name": name, "input": input}, nil
	})
	out, err := fn.Execute(context.Background(), "demo.greet", "hi")
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["name"] != domain.CommandName("demo.greet") || m["input"] != "hi" {
		t.Fatalf("got %#v", m)
	}
}
