package domain_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

// A caller-bound executor acts as the caller on its invocation, never one it
// picks itself.
func TestCallerExecutorFunc_ActsAsInvocationCaller(t *testing.T) {
	caller, err := domain.NewCaller("aux", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}
	var got domain.Caller
	exec := domain.CallerExecutorFunc(func(_ context.Context, c domain.Caller, _ any) (any, error) {
		got = c
		return nil, nil
	})
	ctx := domain.WithInvocation(context.Background(), domain.Invocation{Caller: caller, Command: "project.open"})
	if _, err := exec.Execute(ctx, "project.open", nil); err != nil {
		t.Fatal(err)
	}
	if got != caller {
		t.Fatalf("executor ran as %+v, want %+v", got, caller)
	}
}

// Called outside the gateway (no authorized invocation), a caller-bound
// executor must refuse rather than guess an identity.
func TestCallerExecutorFunc_FailsClosedWithoutInvocation(t *testing.T) {
	called := false
	exec := domain.CallerExecutorFunc(func(context.Context, domain.Caller, any) (any, error) {
		called = true
		return nil, nil
	})
	_, err := exec.Execute(context.Background(), "project.open", nil)
	if !errors.Is(err, domain.ErrNoInvocation) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestInvocationFrom_EmptyContext(t *testing.T) {
	if _, ok := domain.InvocationFrom(context.Background()); ok {
		t.Fatal("empty context reported an invocation")
	}
}
